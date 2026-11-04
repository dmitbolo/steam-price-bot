.PHONY: help build run test cover vet db-up db-down migrate docker-up docker-down docker-logs

APP_NAME := steam-price-bot
BIN_DIR := ./bin

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# --- Local Development ---

build: ## Build the bot binary into ./bin
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/bot

run: ## Run the bot locally on the host machine
	go run ./cmd/bot

db-up: ## Start only PostgreSQL in Docker (for local development)
	docker compose up -d postgres

db-down: ## Stop local Docker containers
	docker compose down

migrate: ## Apply SQL migrations directly to the running DB container
	docker exec -i steam_price_bot_db psql -U postgres -d steam_bot < migrations/001_init.sql

# --- Testing & Quality Assurance ---

test: ## Run unit tests with the race detector
	go test -v -race ./...

cover: ## Run tests and display code coverage percentage per package
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	rm -f coverage.out

vet: ## Analyze the code using go vet static analyzer
	go vet ./...

# --- Full Docker Deployment (DB + Bot) ---

docker-up: ## Build images and start the entire stack (DB + Bot) in Docker
	docker compose up -d --build

docker-down: ## Stop the entire Docker stack
	docker compose down

docker-logs: ## Follow the bot container logs in real time
	docker compose logs -f bot