package okx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OKXTickerInfo defines relevant fields from OKX futures ticker
type OKXTickerInfo struct {
	InstrumentID string `json:"instId"`
	LastPrice    string `json:"last"`
	Open24h      string `json:"open24h"`
	High24h      string `json:"high24h"`
	Low24h       string `json:"low24h"`
	Vol24h       string `json:"vol24h"`
	Change24hPct string `json:"change24h"` // calculated from open/last if not provided
}

// shared rate-limited client instance
var sharedClient = NewRateLimitedClient(&http.Client{
	Timeout: 10 * time.Second,
})

// GetAllTickers fetches all perpetual (swap) futures tickers from OKX
func GetAllTickers() ([]*OKXTickerInfo, error) {
	url := "https://www.okx.com/api/v5/market/tickers?instType=SWAP"
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
		Code string           `json:"code"`
		Msg  string           `json:"msg"`
		Data []*OKXTickerInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return parsed.Data, nil
}

// GetTickerInfo fetches ticker info for a specific instrument (e.g., BTC-USDT-SWAP)
func GetTickerInfo(instID string) (*OKXTickerInfo, error) {
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/ticker?instId=%s", instID)

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
		Code string           `json:"code"`
		Msg  string           `json:"msg"`
		Data []*OKXTickerInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("no data returned for symbol %s", instID)
	}
	return parsed.Data[0], nil
}
