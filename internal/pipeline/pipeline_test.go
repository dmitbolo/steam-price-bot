package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dmitbolo/steam-price-bot/internal/domain"
)

type mockStorage struct {
	mu      sync.Mutex
	tasks   []domain.GameWatchTask
	updated map[int64]int
	err     error
}

func (m *mockStorage) UpsertSubscription(ctx context.Context, sub domain.Subscription) error {
	return nil
}

func (m *mockStorage) GetActiveTasks(ctx context.Context) ([]domain.GameWatchTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}

	tasksCopy := make([]domain.GameWatchTask, len(m.tasks))
	for i, t := range m.tasks {
		subsCopy := make([]domain.Subscription, len(t.Subscriptions))
		copy(subsCopy, t.Subscriptions)
		t.Subscriptions = subsCopy
		tasksCopy[i] = t
	}
	return tasksCopy, nil
}

func (m *mockStorage) UpdateTargetPrice(ctx context.Context, subID int64, newPrice int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated[subID] = newPrice

	for ti := range m.tasks {
		for si := range m.tasks[ti].Subscriptions {
			if m.tasks[ti].Subscriptions[si].ID == subID {
				m.tasks[ti].Subscriptions[si].TargetPrice = newPrice
			}
		}
	}
	return nil
}

func (m *mockStorage) GetUpdatedPrice(subID int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updated[subID]
}

type mockSteamClient struct {
	mu     sync.Mutex
	prices map[int]int
	err    error
}

func (m *mockSteamClient) GetGameInfo(ctx context.Context, appID int) (string, int, error) {
	return "", 0, nil
}

func (m *mockSteamClient) GetBatchPrices(ctx context.Context, appIDs []int) (map[int]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}

	res := make(map[int]int, len(appIDs))
	for _, id := range appIDs {
		if p, ok := m.prices[id]; ok {
			res[id] = p
		}
	}
	return res, nil
}

func (m *mockSteamClient) SetPrice(appID, price int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prices[appID] = price
}

type mockNotifier struct {
	mu     sync.Mutex
	events []domain.NotificationEvent
}

func (m *mockNotifier) SendPriceAlert(ctx context.Context, event domain.NotificationEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockNotifier) ResetEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

func (m *mockNotifier) GetEvents() []domain.NotificationEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]domain.NotificationEvent, len(m.events))
	copy(cp, m.events)
	return cp
}

// --- Tests ---

func TestChunkTasks(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		chunkSize int
		wantLens  []int
	}{
		{
			name:      "empty slice",
			total:     0,
			chunkSize: 100,
			wantLens:  nil,
		},
		{
			name:      "less than chunk size",
			total:     42,
			chunkSize: 100,
			wantLens:  []int{42},
		},
		{
			name:      "exact multiple",
			total:     200,
			chunkSize: 100,
			wantLens:  []int{100, 100},
		},
		{
			name:      "with remainder",
			total:     205,
			chunkSize: 100,
			wantLens:  []int{100, 100, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := make([]domain.GameWatchTask, tt.total)
			for i := range tasks {
				tasks[i] = domain.GameWatchTask{AppID: i + 1}
			}

			chunks := chunkTasks(tasks, tt.chunkSize)

			if len(chunks) != len(tt.wantLens) {
				t.Fatalf("expected %d chunks, got %d", len(tt.wantLens), len(chunks))
			}
			for i, expectedLen := range tt.wantLens {
				if len(chunks[i]) != expectedLen {
					t.Errorf("chunk %d: expected size %d, got %d", i, expectedLen, len(chunks[i]))
				}
			}
		})
	}
}

func TestPricePipeline_RunCheckCycle_Lifecycle(t *testing.T) {
	st := &mockStorage{
		updated: make(map[int64]int),
		tasks: []domain.GameWatchTask{
			{
				AppID:    1091500,
				GameName: "Cyberpunk 2077",
				Subscriptions: []domain.Subscription{
					{ID: 1, ChatID: 111, AppID: 1091500, TargetPrice: 1500},
					{ID: 2, ChatID: 222, AppID: 1091500, TargetPrice: 900},
				},
			},
		},
	}

	sc := &mockSteamClient{
		prices: map[int]int{1091500: 1000},
	}
	nf := &mockNotifier{}
	pipe := NewPricePipeline(st, sc, nf, time.Millisecond)
	ctx := context.Background()

	t.Run("first drop triggers alert for matching subscriber", func(t *testing.T) {
		pipe.RunCheckCycle(ctx)

		events := nf.GetEvents()
		if len(events) != 1 {
			t.Fatalf("expected 1 notification, got %d", len(events))
		}

		alert := events[0]
		if alert.ChatID != 111 || alert.OldPrice != 1500 || alert.NewPrice != 1000 {
			t.Errorf("unexpected alert payload: %+v", alert)
		}

		if st.GetUpdatedPrice(1) != 1000 {
			t.Errorf("expected subscription #1 target price to be updated to 1000, got %d", st.GetUpdatedPrice(1))
		}
	})

	t.Run("same price does not trigger duplicate alert", func(t *testing.T) {
		nf.ResetEvents()
		pipe.RunCheckCycle(ctx)

		events := nf.GetEvents()
		if len(events) != 0 {
			t.Fatalf("expected 0 notifications, got %d", len(events))
		}
	})

	t.Run("further drop triggers alert for all eligible subscribers", func(t *testing.T) {
		nf.ResetEvents()
		sc.SetPrice(1091500, 800)

		pipe.RunCheckCycle(ctx)

		events := nf.GetEvents()
		if len(events) != 2 {
			t.Fatalf("expected 2 notifications, got %d", len(events))
		}
	})

	t.Run("price increase does not trigger alert", func(t *testing.T) {
		nf.ResetEvents()
		sc.SetPrice(1091500, 2000)

		pipe.RunCheckCycle(ctx)

		events := nf.GetEvents()
		if len(events) != 0 {
			t.Fatalf("expected 0 notifications on price increase, got %d", len(events))
		}
	})
}
