package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/dmitbolo/steam-price-bot/internal/pipeline"
	"github.com/dmitbolo/steam-price-bot/internal/steam"
	"github.com/dmitbolo/steam-price-bot/internal/storage"
	"github.com/dmitbolo/steam-price-bot/internal/telegram"
)

const (
	defaultSteamCountry  = "ru"
	defaultCheckInterval = 24 * time.Hour
	defaultBatchRate     = 2 * time.Second
	defaultHTTPTimeout   = 15 * time.Second
)

func main() {
	_ = godotenv.Load()

	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	steamCountry := os.Getenv("STEAM_COUNTRY")
	if steamCountry == "" {
		steamCountry = defaultSteamCountry
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbStorage, err := storage.NewPostgresStorage(ctx, dbDSN)
	if err != nil {
		log.Fatalf("failed to connect to storage: %v", err)
	}
	defer dbStorage.Close()

	steamClient := steam.NewClient(steamCountry, defaultHTTPTimeout)

	tgBot, err := telegram.NewBot(botToken, dbStorage, steamClient)
	if err != nil {
		log.Fatalf("failed to create telegram bot: %v", err)
	}

	pricePipeline := pipeline.NewPricePipeline(dbStorage, steamClient, tgBot, defaultBatchRate)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runDailyScheduler(ctx, pricePipeline, defaultCheckInterval)
	}()

	tgBot.Start(ctx)

	log.Println("[Main] shutting down gracefully, waiting for background workers...")
	wg.Wait()
	log.Println("[Main] application stopped cleanly.")
}

func runDailyScheduler(ctx context.Context, p *pipeline.PricePipeline, interval time.Duration) {
	p.RunCheckCycle(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Scheduler] stopping daily price scheduler...")
			return
		case <-ticker.C:
			p.RunCheckCycle(ctx)
		}
	}
}