package bybit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BybitTickerInfo defines relevant fields from Bybit's ticker response
type BybitTickerInfo struct {
	Symbol       string `json:"symbol"`
	LastPrice    string `json:"lastPrice"`
	Price24hPcnt string `json:"price24hPcnt"`
	HighPrice24h string `json:"highPrice24h"`
	LowPrice24h  string `json:"lowPrice24h"`
	PrevPrice24h string `json:"prevPrice24h"`
	Turnover24h  string `json:"turnover24h"`
	Volume24h    string `json:"volume24h"`
}

// shared rate-limited client instance
var sharedClient = NewRateLimitedClient(&http.Client{
	Timeout: 10 * time.Second,
})

// GetAllTickers fetches all USDT perpetual tickers from Bybit
func GetAllTickers() ([]*BybitTickerInfo, error) {
	url := "https://api.bybit.com/v5/market/tickers?category=linear"

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
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []*BybitTickerInfo `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	if parsed.RetCode != 0 {
		return nil, fmt.Errorf("API error: %s", parsed.RetMsg)
	}

	return parsed.Result.List, nil
}

// GetTickerInfo fetches ticker info for a specific USDT symbol on Bybit
func GetTickerInfo(symbol string) (*BybitTickerInfo, error) {
	url := fmt.Sprintf("https://api.bybit.com/v5/market/ticker?category=linear&symbol=%s", symbol)

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
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []*BybitTickerInfo `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	if parsed.RetCode != 0 {
		return nil, fmt.Errorf("API error: %s", parsed.RetMsg)
	}

	if len(parsed.Result.List) == 0 {
		return nil, fmt.Errorf("no data returned for symbol %s", symbol)
	}

	return parsed.Result.List[0], nil
}
