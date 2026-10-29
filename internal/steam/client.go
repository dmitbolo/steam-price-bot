package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://store.steampowered.com"
	MaxBatchSize   = 100
)

var (
	ErrGameNotFound  = errors.New("game not found or unavailable in steam")
	ErrNoPriceData   = errors.New("game has no price (free-to-play or unreleased)")
	ErrBatchTooLarge = errors.New("batch size exceeds maximum limit of 100 app IDs")
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	country    string
}

func NewClient(country string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    defaultBaseURL,
		country:    country,
	}
}

// Response Steam API: {"1091500": {"success": true, "data": {"name": "...", "price_overview": {"final": 199900}}}}
type appDetailsResponse map[string]struct {
	Success bool `json:"success"`
	Data    struct {
		Name          string `json:"name"`
		PriceOverview *struct {
			Final int `json:"final"`
		} `json:"price_overview"`
	} `json:"data"`
}

// GetGameInfo requests the name and price of a single game (used when adding a subscription).
func (c *Client) GetGameInfo(ctx context.Context, appID int) (string, int, error) {
	url := fmt.Sprintf(
		"%s/api/appdetails?appids=%d&cc=%s&filters=basic,price_overview",
		c.baseURL, appID, c.country,
	)

	data, err := c.doRequest(ctx, url)
	if err != nil {
		return "", 0, err
	}

	appKey := strconv.Itoa(appID)
	entry, ok := data[appKey]
	if !ok || !entry.Success {
		return "", 0, fmt.Errorf("%w: app_id %d", ErrGameNotFound, appID)
	}

	if entry.Data.PriceOverview == nil {
		return "", 0, fmt.Errorf("%w: app_id %d", ErrNoPriceData, appID)
	}

	price := entry.Data.PriceOverview.Final / 100
	return entry.Data.Name, price, nil
}

func (c *Client) doRequest(ctx context.Context, url string) (appDetailsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result appDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// GetBatchPrices requests current prices for a batch of games simultaneously (up to 100 items per single HTTP request).
// Returns map[appID]price.
func (c *Client) GetBatchPrices(ctx context.Context, appIDs []int) (map[int]int, error) {
	if len(appIDs) == 0 {
		return make(map[int]int), nil
	}
	if len(appIDs) > MaxBatchSize {
		return nil, fmt.Errorf("%w: got %d", ErrBatchTooLarge, len(appIDs))
	}

	strIDs := make([]string, len(appIDs))
	for i, id := range appIDs {
		strIDs[i] = strconv.Itoa(id)
	}

	url := fmt.Sprintf(
		"%s/api/appdetails?appids=%s&cc=%s&filters=price_overview",
		c.baseURL, strings.Join(strIDs, ","), c.country,
	)

	data, err := c.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	prices := make(map[int]int, len(appIDs))
	for _, appID := range appIDs {
		appKey := strconv.Itoa(appID)
		entry, ok := data[appKey]
		if !ok || !entry.Success || entry.Data.PriceOverview == nil {
			continue
		}
		prices[appID] = entry.Data.PriceOverview.Final / 100
	}

	return prices, nil
}