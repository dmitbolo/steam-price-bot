package storage

import (
	"reflect"
	"testing"

	"github.com/dmitbolo/steam-price-bot/internal/domain"
)

func TestGroupSubscriptionsByAppID(t *testing.T) {
	tests := []struct {
		name string
		subs []domain.Subscription
		want []domain.GameWatchTask
	}{
		{
			name: "empty input",
			subs: nil,
			want: nil,
		},
		{
			name: "successful grouping",
			subs: []domain.Subscription{
				{ID: 1, ChatID: 101, AppID: 1091500, GameName: "Cyberpunk 2077", TargetPrice: 1500},
				{ID: 2, ChatID: 202, AppID: 1091500, GameName: "Cyberpunk 2077", TargetPrice: 1200},
				{ID: 3, ChatID: 101, AppID: 413150, GameName: "Stardew Valley", TargetPrice: 300},
			},
			want: []domain.GameWatchTask{
				{
					AppID:    1091500,
					GameName: "Cyberpunk 2077",
					Subscriptions: []domain.Subscription{
						{ID: 1, ChatID: 101, AppID: 1091500, GameName: "Cyberpunk 2077", TargetPrice: 1500},
						{ID: 2, ChatID: 202, AppID: 1091500, GameName: "Cyberpunk 2077", TargetPrice: 1200},
					},
				},
				{
					AppID:    413150,
					GameName: "Stardew Valley",
					Subscriptions: []domain.Subscription{
						{ID: 3, ChatID: 101, AppID: 413150, GameName: "Stardew Valley", TargetPrice: 300},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupSubscriptionsByAppID(tt.subs)
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("groupSubscriptionsByAppID() = %v, want %v", got, tt.want)
			}
		})
	}
}
