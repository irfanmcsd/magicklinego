package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BitgetTickerInfo represents the ticker data structure from Bitget
type BitgetTickerInfo struct {
	Symbol           string `json:"symbol"`        // e.g., BTCUSDT
	LastPrice        string `json:"last"`          // Latest price
	High24h          string `json:"high24h"`       // 24h high
	Low24h           string `json:"low24h"`        // 24h low
	Open24h          string `json:"open24h"`       // 24h open price
	Change24hPercent string `json:"changePercent"` // % change in last 24h
	BaseVolume       string `json:"baseVolume"`    // Volume in base currency
	QuoteVolume      string `json:"quoteVolume"`   // Volume in quote currency
	Timestamp        string `json:"ts"`            // Timestamp
}

// shared rate-limited client instance
var sharedClient = NewRateLimitedClient(&http.Client{
	Timeout: 10 * time.Second,
})

// GetAllTickers fetches all USDT-margined perpetual futures tickers from Bitget
func GetAllTickers() ([]*BitgetTickerInfo, error) {
	url := "https://api.bitget.com/api/v2/market/tickers?productType=umcbl"

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

	var parsed struct {
		Code string              `json:"code"`
		Msg  string              `json:"msg"`
		Data []*BitgetTickerInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Code != "00000" {
		return nil, fmt.Errorf("bitget API error: %s", parsed.Msg)
	}
	return parsed.Data, nil
}

// GetTickerInfo fetches a specific ticker for a given symbol like BTCUSDT
func GetTickerInfo(symbol string) (*BitgetTickerInfo, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/market/ticker?symbol=%s", symbol)

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

	var parsed struct {
		Code string              `json:"code"`
		Msg  string              `json:"msg"`
		Data []*BitgetTickerInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Code != "00000" {
		return nil, fmt.Errorf("bitget API error: %s", parsed.Msg)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("no data returned for symbol %s", symbol)
	}
	return parsed.Data[0], nil
}
