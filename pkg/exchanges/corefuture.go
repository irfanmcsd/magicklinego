package exchanges

import (
	"errors"
	"strings"

	"scanner.magictradebot.com/pkg/binance"
	"scanner.magictradebot.com/pkg/bitget"
	"scanner.magictradebot.com/pkg/bybit"
	"scanner.magictradebot.com/pkg/okx"
)

// TickerInfo is a generic struct for normalized ticker data across exchanges
type TickerInfo struct {
	Symbol    string
	LastPrice string
	High24h   string
	Low24h    string
	Vol24h    string
	Change24h string
	Exchange  string
}

// CoreFuturesAllTickers fetches all tickers from the specified exchange
func CoreFuturesAllTickers(exchange string) ([]*TickerInfo, error) {
	ex := strings.ToLower(exchange)
	switch ex {
	case "binance":
		data, err := binance.GetAllTickers()
		if err != nil {
			return nil, err
		}
		result := make([]*TickerInfo, 0, len(data))
		for _, t := range data {
			result = append(result, &TickerInfo{
				Symbol:    t.Symbol,
				LastPrice: t.LastPrice,
				High24h:   "",
				Low24h:    "",
				Vol24h:    t.Volume,
				Change24h: t.PriceChangePercent,
				Exchange:  "binance",
			})
		}
		return result, nil

	case "okx":
		data, err := okx.GetAllTickers()
		if err != nil {
			return nil, err
		}
		result := make([]*TickerInfo, 0, len(data))
		for _, t := range data {
			result = append(result, &TickerInfo{
				Symbol:    t.InstrumentID,
				LastPrice: t.LastPrice,
				High24h:   t.High24h,
				Low24h:    t.Low24h,
				Vol24h:    t.Vol24h,
				Change24h: t.Change24hPct,
				Exchange:  "okx",
			})
		}
		return result, nil

	case "bitget":
		data, err := bitget.GetAllTickers()
		if err != nil {
			return nil, err
		}
		result := make([]*TickerInfo, 0, len(data))
		for _, t := range data {
			result = append(result, &TickerInfo{
				Symbol:    t.Symbol,
				LastPrice: t.LastPrice,
				High24h:   t.High24h,
				Low24h:    t.Low24h,
				Vol24h:    t.BaseVolume,
				Change24h: t.Change24hPercent,
				Exchange:  "bitget",
			})
		}
		return result, nil

	case "bybit":
		data, err := bybit.GetAllTickers()
		if err != nil {
			return nil, err
		}
		result := make([]*TickerInfo, 0, len(data))
		for _, t := range data {
			result = append(result, &TickerInfo{
				Symbol:    t.Symbol,
				LastPrice: t.LastPrice,
				High24h:   t.HighPrice24h,
				Low24h:    t.LowPrice24h,
				Vol24h:    t.Volume24h,
				Change24h: t.Price24hPcnt,
				Exchange:  "bybit",
			})
		}
		return result, nil

	default:
		return nil, errors.New("unsupported exchange: " + exchange)
	}
}
