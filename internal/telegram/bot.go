package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/dmitbolo/steam-price-bot/internal/domain"
	"github.com/dmitbolo/steam-price-bot/internal/parser"
	"github.com/dmitbolo/steam-price-bot/internal/steam"
)

var _ domain.Notifier = (*Bot)(nil)

type Bot struct {
	client  *bot.Bot
	storage domain.Storage
	steam   domain.SteamClient
}

func NewBot(token string, st domain.Storage, sc domain.SteamClient) (*Bot, error) {
	b := &Bot{
		storage: st,
		steam:   sc,
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.handleMessage),
	}

	tgClient, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}

	b.client = tgClient
	return b, nil
}

// Start launches the long-polling update loop from Telegram. It blocks until ctx is canceled.
func (b *Bot) Start(ctx context.Context) {
	log.Println("[Telegram] bot started polling...")
	b.client.Start(ctx)
	log.Println("[Telegram] bot stopped.")
}

// SendPriceAlert implements domain.Notifier, serving as the 4th stage (Sink) of the pipeline.
func (b *Bot) SendPriceAlert(ctx context.Context, event domain.NotificationEvent) error {
	text := fmt.Sprintf(
		"🔥 Снижение цены на *%s*!\n\n"+
			"Старый порог: %d\n"+
			"Новая цена: *%d*\n\n"+
			"https://store.steampowered.com/app/%d",
		escapeMarkdown(event.GameName),
		event.OldPrice,
		event.NewPrice,
		event.AppID,
	)

	_, err := b.client.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    event.ChatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}

	return nil
}

func (b *Bot) handleMessage(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	chatID := update.Message.Chat.ID
	replyText := b.processUserMessage(ctx, chatID, update.Message.Text)

	if _, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
	}); err != nil {
		log.Printf("[Telegram] failed to send reply to chat %d: %v", chatID, err)
	}
}

// processUserMessage contains the core business logic for handling incoming text commands.
func (b *Bot) processUserMessage(ctx context.Context, chatID int64, text string) string {
	trimmed := strings.TrimSpace(text)

	if trimmed == "/start" || trimmed == "/help" {
		return "Привет! Отправь мне ссылку на игру в Steam и желаемую цену через пробел.\n\n" +
			"Пример:\nhttps://store.steampowered.com/app/1091500/Cyberpunk_2077/ 1500"
	}

	req, err := parser.ParseUserMessage(trimmed)
	if err != nil {
		return "Неверный формат. Отправь ссылку на игру в Steam и желаемую цену (целое число) через пробел.\n\n" +
			"Пример:\nhttps://store.steampowered.com/app/1091500/ 1500"
	}

	gameName, currentPrice, err := b.steam.GetGameInfo(ctx, req.AppID)
	if err != nil {
		if errors.Is(err, steam.ErrGameNotFound) {
			return "Игра не найдена в Steam или недоступна в текущем регионе."
		}
		if errors.Is(err, steam.ErrNoPriceData) {
			return "У этой игры нет цены в Steam (возможно, она бесплатная или еще не вышла)."
		}
		log.Printf("[Telegram] steam api error for app %d: %v", req.AppID, err)
		return "Не удалось проверить игру в Steam. Попробуй чуть позже."
	}

	sub := domain.Subscription{
		ChatID:      chatID,
		AppID:       req.AppID,
		GameName:    gameName,
		TargetPrice: req.TargetPrice,
	}

	if err := b.storage.UpsertSubscription(ctx, sub); err != nil {
		log.Printf("[Telegram] db error saving subscription for chat %d: %v", chatID, err)
		return "Произошла ошибка при сохранении подписки."
	}

	return fmt.Sprintf(
		"Подписка оформлена!\nИгра: %s\nТекущая цена в Steam: %d\nУведомлю, когда цена упадет ниже: %d",
		gameName, currentPrice, req.TargetPrice,
	)
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "`", "\\`")
	return replacer.Replace(s)
}