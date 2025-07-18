package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// example struct for parsed response
type TickerInfo struct {
	Symbol             string `json:"symbol"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice          string `json:"lastPrice"`
	Volume             string `json:"volume"`
}

// shared rate-limited client instance
var sharedClient = NewRateLimitedClient(&http.Client{
	Timeout: 10 * time.Second,
})

func GetAllTickers() ([]*TickerInfo, error) {
	url := "https://fapi.binance.com/fapi/v1/ticker/24hr"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := sharedClient.SendWithRetry(req, 1)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response: %d\n%s", resp.StatusCode, string(body))
	}

	var tickers []*TickerInfo
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, err
	}
	return tickers, nil
}

func GetTickerInfo(symbol string) (*TickerInfo, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := sharedClient.SendWithRetry(req, 1) // ✅ now using instance method
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response: %d\n%s", resp.StatusCode, string(body))
	}

	var data TickerInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
