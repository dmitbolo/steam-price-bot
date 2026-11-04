package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/dmitbolo/steam-price-bot/internal/domain"
)

type PostgresStorage struct {
	pool *pgxpool.Pool
}

// // NewPostgresStorage creates a PostgreSQL connection pool and checks database availability.
func NewPostgresStorage(ctx context.Context, dsn string) (*PostgresStorage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresStorage{pool: pool}, nil
}

// Close gracefully closes the connection pool when the application stops.
func (s *PostgresStorage) Close() {
	s.pool.Close()
}

// UpsertSubscription saves a new subscription or updates the target price
// if the user is already subscribed to this specific game.
func (s *PostgresStorage) UpsertSubscription(ctx context.Context, sub domain.Subscription) error {
	const query = `
		INSERT INTO subscriptions (chat_id, app_id, game_name, target_price, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (chat_id, app_id) DO UPDATE
		SET game_name    = EXCLUDED.game_name,
		    target_price = EXCLUDED.target_price,
		    updated_at   = NOW();
	`

	_, err := s.pool.Exec(ctx, query, sub.ChatID, sub.AppID, sub.GameName, sub.TargetPrice)
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}

	return nil
}

// GetActiveTasks fetches all subscriptions from the database and groups them by AppID,
// ensuring the pipeline requests each game from the Steam API only once.
func (s *PostgresStorage) GetActiveTasks(ctx context.Context) ([]domain.GameWatchTask, error) {
	const query = `
		SELECT id, chat_id, app_id, game_name, target_price
		FROM subscriptions
		ORDER BY app_id, id;
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(&sub.ID, &sub.ChatID, &sub.AppID, &sub.GameName, &sub.TargetPrice); err != nil {
			return nil, fmt.Errorf("scan subscription row: %w", err)
		}
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription rows: %w", err)
	}

	return groupSubscriptionsByAppID(subs), nil
}

// groupSubscriptionsByAppID aggregates a flat slice of subscriptions into tasks grouped by a unique AppID,
// preserving the deterministic order of the games.
func groupSubscriptionsByAppID(subs []domain.Subscription) []domain.GameWatchTask {
	if len(subs) == 0 {
		return nil
	}

	taskIndexByAppID := make(map[int]int, len(subs))
	var tasks []domain.GameWatchTask

	for _, sub := range subs {
		idx, exists := taskIndexByAppID[sub.AppID]
		if !exists {
			idx = len(tasks)
			taskIndexByAppID[sub.AppID] = idx
			tasks = append(tasks, domain.GameWatchTask{
				AppID:    sub.AppID,
				GameName: sub.GameName,
			})
		}
		tasks[idx].Subscriptions = append(tasks[idx].Subscriptions, sub)
	}

	return tasks
}