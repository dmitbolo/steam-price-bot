package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/dmitbolo/steam-price-bot/internal/domain"
	"github.com/dmitbolo/steam-price-bot/internal/steam"
)

var _ domain.Storage = (*mockStorage)(nil)
var _ domain.SteamClient = (*mockSteam)(nil)

type mockStorage struct {
	saved []domain.Subscription
}

func (m *mockStorage) UpsertSubscription(ctx context.Context, sub domain.Subscription) error {
	m.saved = append(m.saved, sub)
	return nil
}

func (m *mockStorage) GetActiveTasks(ctx context.Context) ([]domain.GameWatchTask, error) {
	return nil, nil
}

func (m *mockStorage) UpdateTargetPrice(ctx context.Context, subID int64, newPrice int) error {
	return nil
}

type mockSteam struct {
	name  string
	price int
	err   error
}

func (m *mockSteam) GetGameInfo(ctx context.Context, appID int) (string, int, error) {
	return m.name, m.price, m.err
}

func (m *mockSteam) GetBatchPrices(ctx context.Context, appIDs []int) (map[int]int, error) {
	return nil, nil
}

func TestBot_ProcessUserMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		inputText     string
		chatID        int64
		steamMock     mockSteam
		wantSubscript bool
		expectedSub   domain.Subscription // Checked only if wantSubscript is true
		wantInReply   string
	}{
		{
			name:          "handles /start command",
			inputText:     "/start",
			chatID:        100,
			steamMock:     mockSteam{},
			wantSubscript: false,
			wantInReply:   "Привет!",
		},
		{
			name:          "saves valid subscription after checking steam",
			inputText:     "https://store.steampowered.com/app/1091500 1500",
			chatID:        777,
			steamMock:     mockSteam{name: "Cyberpunk 2077", price: 1999},
			wantSubscript: true,
			expectedSub: domain.Subscription{
				ChatID:      777,
				AppID:       1091500,
				GameName:    "Cyberpunk 2077",
				TargetPrice: 1500,
			},
			wantInReply: "Подписка оформлена!",
		},
		{
			name:          "handles free game error from steam",
			inputText:     "https://store.steampowered.com/app/570 500",
			chatID:        777,
			steamMock:     mockSteam{err: steam.ErrNoPriceData},
			wantSubscript: false,
			wantInReply:   "нет цены",
		},
		{
			name:          "handles invalid message format",
			inputText:     "invalid_text_without_price",
			chatID:        777,
			steamMock:     mockSteam{},
			wantSubscript: false,
			wantInReply:   "Неверный формат",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &mockStorage{}
			b := &Bot{
				storage: st,
				steam:   &tt.steamMock,
			}

			reply := b.processUserMessage(ctx, tt.chatID, tt.inputText)

			// 1. Assert the text response content
			if !strings.Contains(reply, tt.wantInReply) {
				t.Errorf("processUserMessage() reply = %q, want to contain %q", reply, tt.wantInReply)
			}

			// 2. Assert database state mutations
			if tt.wantSubscript {
				if len(st.saved) != 1 {
					t.Fatalf("expected 1 saved subscription, got %d", len(st.saved))
				}
				gotSub := st.saved[0]
				if gotSub != tt.expectedSub {
					t.Errorf("saved subscription = %+v, want %+v", gotSub, tt.expectedSub)
				}
			} else {
				if len(st.saved) != 0 {
					t.Errorf("expected 0 saved subscriptions, got %d", len(st.saved))
				}
			}
		})
	}
}