package steam

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetGameInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID := r.URL.Query().Get("appids")

		switch appID {
		case "1091500":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"1091500": {
					"success": true,
					"data": {
						"name": "Cyberpunk 2077",
						"price_overview": { "final": 199900 }
					}
				}
			}`))
		case "570": // Dota 2 (free, without price_overview)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"570": {
					"success": true,
					"data": { "name": "Dota 2" }
				}
			}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"999999": {"success": false}}`))
		}
	}))
	defer server.Close()

	client := NewClient("ru", 2*time.Second)
	client.baseURL = server.URL

	t.Run("successfully fetches paid game info", func(t *testing.T) {
		name, price, err := client.GetGameInfo(context.Background(), 1091500)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "Cyberpunk 2077" {
			t.Errorf("got name %q, want %q", name, "Cyberpunk 2077")
		}
		if price != 1999 {
			t.Errorf("got price %d, want 1999", price)
		}
	})

	t.Run("returns ErrNoPriceData for free game", func(t *testing.T) {
		_, _, err := client.GetGameInfo(context.Background(), 570)
		if !errors.Is(err, ErrNoPriceData) {
			t.Errorf("got err %v, want %v", err, ErrNoPriceData)
		}
	})

	t.Run("returns ErrGameNotFound when success is false", func(t *testing.T) {
		_, _, err := client.GetGameInfo(context.Background(), 999999)
		if !errors.Is(err, ErrGameNotFound) {
			t.Errorf("got err %v, want %v", err, ErrGameNotFound)
		}
	})
}

func TestClient_GetBatchPrices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filters") != "price_overview" {
			t.Errorf("expected filters=price_overview for batch request, got %s", r.URL.Query().Get("filters"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"1091500": {
				"success": true,
				"data": { "price_overview": { "final": 150000 } }
			},
			"413150": {
				"success": true,
				"data": { "price_overview": { "final": 45000 } }
			},
			"570": {
				"success": true,
				"data": {}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("ru", 2*time.Second)
	client.baseURL = server.URL

	t.Run("fetches multiple prices and skips free games", func(t *testing.T) {
		prices, err := client.GetBatchPrices(context.Background(), []int{1091500, 413150, 570})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(prices) != 2 {
			t.Fatalf("expected 2 prices in map, got %d", len(prices))
		}
		if prices[1091500] != 1500 {
			t.Errorf("expected price 1500 for 1091500, got %d", prices[1091500])
		}
		if prices[413150] != 450 {
			t.Errorf("expected price 450 for 413150, got %d", prices[413150])
		}
	})

	t.Run("rejects batch larger than 100 items", func(t *testing.T) {
		oversized := make([]int, 101)
		_, err := client.GetBatchPrices(context.Background(), oversized)
		if !errors.Is(err, ErrBatchTooLarge) {
			t.Errorf("expected ErrBatchTooLarge, got %v", err)
		}
	})
}