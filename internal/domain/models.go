package domain

import "context"

// Subscription represents a record of a user's subscription to a game
type Subscription struct {
	ID          int64
	ChatID      int64
	AppID       int
	GameName    string
	TargetPrice int
}

// GameWatchTask represents a grouped task to check a single game for all subscribed users
type GameWatchTask struct {
	AppID         int
	GameName      string
	Subscriptions []Subscription
}

// EnrichedGameTask represents a game with current price, retrieved from the Steam API
type EnrichedGameTask struct {
	Task         GameWatchTask
	CurrentPrice int
}

// NotificationEvent represents an event for sending a notification to user
type NotificationEvent struct {
	ChatID   int64
	AppID    int
	GameName string
	OldPrice int
	NewPrice int
}

// Storage represents a contract for working with a database
type Storage interface {
	UpsertSubscription(ctx context.Context, sub Subscription) error
	GetActiveTasks(ctx context.Context) ([]GameWatchTask, error)
	UpdateTargetPrice(ctx context.Context, subID int64, newPrice int) error
}

// SteamClient represents contract for working with a Steam API.
type SteamClient interface {
	// GetGameInfo fetches the name and current price for a single game
	GetGameInfo(ctx context.Context, appID int) (name string, price int, err error)
	// Accepts up to 100 appIDs, returns map[appID]price
	GetBatchPrices(ctx context.Context, appIDs []int) (map[int]int, error)
}

// Notifier represents contract for sending notifications to a user
type Notifier interface {
	SendPriceAlert(ctx context.Context, event NotificationEvent) error
}