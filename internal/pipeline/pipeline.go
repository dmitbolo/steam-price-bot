package pipeline

import (
	"context"
	"log"
	"time"

	"github.com/dmitbolo/steam-price-bot/internal/domain"
	"github.com/dmitbolo/steam-price-bot/internal/steam"
)

type PricePipeline struct {
	storage   domain.Storage
	steam     domain.SteamClient
	notifier  domain.Notifier
	batchSize int
	rateLimit time.Duration
}

func NewPricePipeline(
	st domain.Storage,
	sc domain.SteamClient,
	nf domain.Notifier,
	rateLimit time.Duration,
) *PricePipeline {
	return &PricePipeline{
		storage:   st,
		steam:     sc,
		notifier:  nf,
		batchSize: steam.MaxBatchSize, // 100 игр за один запрос
		rateLimit: rateLimit,
	}
}

// stageLoadBatches (Stage 1) loads active games from the storage, 
// splits them into batches of up to 100 items, and sends them to the output channel.
func (p *PricePipeline) stageLoadBatches(ctx context.Context) <-chan []domain.GameWatchTask {
	out := make(chan []domain.GameWatchTask)

	go func() {
		defer close(out)

		tasks, err := p.storage.GetActiveTasks(ctx)
		if err != nil {
			log.Printf("[Pipeline Stage 1] failed to load active tasks: %v", err)
			return
		}

		batches := chunkTasks(tasks, p.batchSize)
		for _, batch := range batches {
			select {
			case <-ctx.Done():
				return
			case out <- batch:
			}
		}
	}()

	return out
}

// stageFetchBatchPrices (Stage 2) reads task batches, requests prices from the Steam API 
// at paced intervals governed by the ticker, and emits enriched game tasks individually to the next channel.
func (p *PricePipeline) stageFetchBatchPrices(
	ctx context.Context,
	in <-chan []domain.GameWatchTask,
) <-chan domain.EnrichedGameTask {
	out := make(chan domain.EnrichedGameTask)

	go func() {
		defer close(out)

		ticker := time.NewTicker(p.rateLimit)
		defer ticker.Stop()

		for batch := range in {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			appIDs := make([]int, len(batch))
			for i, task := range batch {
				appIDs[i] = task.AppID
			}

			prices, err := p.steam.GetBatchPrices(ctx, appIDs)
			if err != nil {
				log.Printf("[Pipeline Stage 2] failed to fetch batch prices (%d games): %v", len(batch), err)
				continue
			}

			for _, task := range batch {
				price, ok := prices[task.AppID]
				if !ok {
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- domain.EnrichedGameTask{
					Task:         task,
					CurrentPrice: price,
				}:
				}
			}
		}
	}()

	return out
}

// chunkTasks splits a slice of tasks into batches of the specified chunk size.
func chunkTasks(tasks []domain.GameWatchTask, chunkSize int) [][]domain.GameWatchTask {
	if len(tasks) == 0 || chunkSize <= 0 {
		return nil
	}

	var chunks [][]domain.GameWatchTask
	for i := 0; i < len(tasks); i += chunkSize {
		end := i + chunkSize
		if end > len(tasks) {
			end = len(tasks)
		}
		chunks = append(chunks, tasks[i:end])
	}

	return chunks
}

// RunCheckCycle triggers all pipeline stages sequentially and blocks until all notifications are fully processed.
func (p *PricePipeline) RunCheckCycle(ctx context.Context) {
	log.Println("[Pipeline] starting daily price check cycle...")

	batchesCh := p.stageLoadBatches(ctx)
	pricesCh := p.stageFetchBatchPrices(ctx, batchesCh)
	alertsCh := p.stageEvaluateAndSave(ctx, pricesCh)

	p.stageSendNotifications(ctx, alertsCh)

	log.Println("[Pipeline] price check cycle completed.")
}

// stageEvaluateAndSave (Stage 3) matches the current price against each subscriber's target price.
// If the price drops below the threshold, it updates the database and yields a notification event.
func (p *PricePipeline) stageEvaluateAndSave(
	ctx context.Context,
	in <-chan domain.EnrichedGameTask,
) <-chan domain.NotificationEvent {
	out := make(chan domain.NotificationEvent)

	go func() {
		defer close(out)

		for item := range in {
			for _, sub := range item.Task.Subscriptions {
				if item.CurrentPrice < sub.TargetPrice {
					if err := p.storage.UpdateTargetPrice(ctx, sub.ID, item.CurrentPrice); err != nil {
						log.Printf("[Pipeline Stage 3] failed to update target price for sub %d: %v", sub.ID, err)
						continue
					}

					event := domain.NotificationEvent{
						ChatID:   sub.ChatID,
						AppID:    sub.AppID,
						GameName: item.Task.GameName,
						OldPrice: sub.TargetPrice,
						NewPrice: item.CurrentPrice,
					}

					select {
					case <-ctx.Done():
						return
					case out <- event:
					}
				}
			}
		}
	}()

	return out
}

// stageSendNotifications (Stage 4 / Sink) consumes events from the channel and transmits alerts via Telegram.
func (p *PricePipeline) stageSendNotifications(ctx context.Context, in <-chan domain.NotificationEvent) {
	for event := range in {
		if ctx.Err() != nil {
			return
		}
		if err := p.notifier.SendPriceAlert(ctx, event); err != nil {
			log.Printf("[Pipeline Stage 4] failed to send alert to chat %d: %v", event.ChatID, err)
		}
	}
}