package parser

import (
	"errors"
	"testing"
)

func TestParseUserMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    UserRequest
		wantErr error
	}{
		{
			name:  "valid full url and price",
			input: "https://store.steampowered.com/app/1091500/Cyberpunk_2077/ 1500",
			want:  UserRequest{AppID: 1091500, TargetPrice: 1500},
		},
		{
			name:  "valid short url with query params and extra spaces",
			input: "   https://store.steampowered.com/app/730?snr=1_7_15__13    500  ",
			want:  UserRequest{AppID: 730, TargetPrice: 500},
		},
		{
			name:    "missing price",
			input:   "https://store.steampowered.com/app/1091500/Cyberpunk_2077/",
			wantErr: ErrInvalidFormat,
		},
		{
			name:    "too many arguments",
			input:   "https://store.steampowered.com/app/1091500 1500 руб",
			wantErr: ErrInvalidFormat,
		},
		{
			name:    "wrong domain or missing app id",
			input:   "https://steamcommunity.com/id/gaben 1500",
			wantErr: ErrInvalidURL,
		},
		{
			name:    "negative price",
			input:   "https://store.steampowered.com/app/1091500 -200",
			wantErr: ErrInvalidPrice,
		},
		{
			name:    "zero price",
			input:   "https://store.steampowered.com/app/1091500 0",
			wantErr: ErrInvalidPrice,
		},
		{
			name:    "non-numeric price",
			input:   "https://store.steampowered.com/app/1091500 free",
			wantErr: ErrInvalidPrice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUserMessage(tt.input)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseUserMessage() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("ParseUserMessage() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}