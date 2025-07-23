package aggregator

import (
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"scanner.magictradebot.com/models"
)

type TickData struct {
	Price  float64
	Time   int64
	Volume float64
}

func (a *KlineAggregator) AddPrice(symbol string, price float64, volume float64) {
	a.lock.Lock()
	defer a.lock.Unlock()

	now := time.Now().UnixMilli()
	truncated := now - (now % 1000)

	tick := TickData{
		Price:  price,
		Time:   truncated,
		Volume: volume,
	}

	// Check if we already have a tick at this timestamp
	exists := false
	for _, t := range a.tickBuffer[symbol] {
		if t.Time == truncated {
			exists = true
			break
		}
	}

	if !exists {
		a.tickBuffer[symbol] = append(a.tickBuffer[symbol], tick)

		if a.Debug {
			a.Logger.Printf("[DEBUG] Added NEW tick for %s: %.4f @ %s",
				symbol, price, time.UnixMilli(truncated).UTC().Format("15:04:05"))
		}
	} else if a.Debug {
		a.Logger.Printf("[DEBUG] Skipped duplicate tick for %s at %s",
			symbol, time.UnixMilli(truncated).UTC().Format("15:04:05"))
	}
}

type KlineAggregator struct {
	tickBuffer    map[string][]TickData
	intervalToMs  map[string]int64
	maxIntervalMs int64 // Track longest interval
	lock          sync.Mutex
	Debug         bool
	Logger        *logrus.Logger
}

func NewKlineAggregator(logger *logrus.Logger, debugMode bool) *KlineAggregator {
	intervals := map[string]int64{
		"1m":  60_000,
		"3m":  3 * 60_000,
		"5m":  5 * 60_000,
		"15m": 15 * 60_000,
		"30m": 30 * 60_000,
		"1h":  60 * 60_000,
		"2h":  2 * 60 * 60_000,
		"4h":  4 * 60 * 60_000,
		"12h": 12 * 60 * 60_000,
		"1d":  24 * 60 * 60_000,
		"2d":  2 * 24 * 60 * 60_000,
		"3d":  3 * 24 * 60 * 60_000,
	}

	// Find max interval
	var maxIntervalMs int64
	for _, ms := range intervals {
		if ms > maxIntervalMs {
			maxIntervalMs = ms
		}
	}

	return &KlineAggregator{
		tickBuffer:    make(map[string][]TickData),
		intervalToMs:  intervals,
		maxIntervalMs: maxIntervalMs,
		Logger:        logger,
		Debug:         debugMode,
	}
}

func (a *KlineAggregator) ExtractOhlc(now int64, intervals ...string) []models.SymbolKlineData {
	a.lock.Lock()
	defer a.lock.Unlock()

	var result []models.SymbolKlineData

	for symbol, ticks := range a.tickBuffer {
		if len(ticks) == 0 {
			continue
		}

		sort.Slice(ticks, func(i, j int) bool {
			return ticks[i].Time < ticks[j].Time
		})

		for _, interval := range intervals {
			intervalMs, ok := a.intervalToMs[interval]
			if !ok {
				continue
			}

			// Precise boundary calculation
			candleEnd := (now / intervalMs) * intervalMs
			candleStart := candleEnd - intervalMs

			// Debug with local time
			if a.Debug {
				startUTC := time.UnixMilli(candleStart).UTC().Format("15:04:05")
				endUTC := time.UnixMilli(candleEnd).UTC().Format("15:04:05")
				startLocal := time.UnixMilli(candleStart).Local().Format("15:04:05")
				endLocal := time.UnixMilli(candleEnd).Local().Format("15:04:05")

				a.Logger.Printf(
					"[DEBUG] %s %s candle: UTC(%s - %s) Local(%s - %s)",
					symbol, interval, startUTC, endUTC, startLocal, endLocal,
				)
			}

			// Find ticks efficiently
			var group []TickData
			for _, tick := range ticks {
				if tick.Time >= candleStart && tick.Time < candleEnd {
					group = append(group, tick)
				}
			}

			if len(group) == 0 {
				if a.Debug {
					a.Logger.Printf(
						"[DEBUG] No ticks for %s %s in [%d-%d]",
						symbol, interval, candleStart, candleEnd,
					)
				}
				continue
			}

			kline := a.buildKline(symbol, interval, group, candleStart)
			result = append(result, kline)
		}

		// Cleanup with buffer
		a.cleanupOldTicks(symbol, now)
	}
	return result
}

func (a *KlineAggregator) cleanupOldTicks(symbol string, now int64) {
	// Add buffer to keep extra history
	buffer := a.maxIntervalMs * 2
	oldestValid := now - buffer

	filtered := []TickData{}
	for _, tick := range a.tickBuffer[symbol] {
		if tick.Time >= oldestValid {
			filtered = append(filtered, tick)
		}
	}
	a.tickBuffer[symbol] = filtered
}

// In buildKline function - use sorted group directly
func (a *KlineAggregator) buildKline(symbol, interval string, group []TickData, openTime int64) models.SymbolKlineData {
	open := group[0].Price
	close := group[len(group)-1].Price
	high := group[0].Price
	low := group[0].Price
	volumeSum := 0.0

	for _, tick := range group {
		if tick.Price > high {
			high = tick.Price
		}
		if tick.Price < low {
			low = tick.Price
		}
		volumeSum += tick.Volume
	}

	return models.SymbolKlineData{
		Symbol:     symbol,
		Interval:   interval,
		Open:       open,
		Close:      close,
		High:       high,
		Low:        low,
		OpenTime:   openTime,
		Volume:     volumeSum,
		TradeCount: int64(len(group)),
	}
}
