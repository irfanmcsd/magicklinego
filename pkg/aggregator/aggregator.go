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

	// Round to nearest second to reduce noise (optional)
	truncated := now - (now % 1000)

	// Append tick data
	tick := TickData{
		Price:  price,
		Time:   truncated,
		Volume: volume,
	}
	a.tickBuffer[symbol] = append(a.tickBuffer[symbol], tick)

	// Log the tick input
	if a.Logger != nil {
		a.Logger.Printf("[DEBUG] Added tick for %s: price=%.4f volume=%.4f time=%d (truncated)", symbol, price, volume, truncated)
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

func (a *KlineAggregator) ExtractOhlc(intervals ...string) []models.SymbolKlineData {
	a.lock.Lock()
	defer a.lock.Unlock()

	now := time.Now().UnixMilli()
	var result []models.SymbolKlineData

	for symbol, ticks := range a.tickBuffer {
		if len(ticks) == 0 {
			continue
		}

		// Sort ticks chronologically
		sort.Slice(ticks, func(i, j int) bool {
			return ticks[i].Time < ticks[j].Time
		})

		for _, interval := range intervals {
			intervalMs, ok := a.intervalToMs[interval]
			if !ok {
				continue
			}

			// Calculate FIXED epoch-aligned boundaries
			candleEnd := (now / intervalMs) * intervalMs
			candleStart := candleEnd - intervalMs

			if a.Debug {
				a.Logger.Printf(
					"[DEBUG] %s %s candle: %s - %s",
					symbol, interval,
					time.UnixMilli(candleStart).UTC().Format("15:04:05"),
					time.UnixMilli(candleEnd).UTC().Format("15:04:05"),
				)
			}

			// Verify we have data covering the full candle period
			if ticks[0].Time > candleStart {
				if a.Debug {
					a.Logger.Printf("[DEBUG] Insufficient data for %s %s: first tick %d > candle start %d",
						symbol, interval, ticks[0].Time, candleStart)
				}
				continue
			}

			// Collect ticks for this candle
			var group []TickData
			for _, tick := range ticks {
				if tick.Time >= candleStart && tick.Time < candleEnd {
					group = append(group, tick)
				}
			}

			if len(group) == 0 {
				continue
			}

			// Build and add the candle
			kline := a.buildKline(symbol, interval, group, candleStart)
			result = append(result, kline)
		}

		// Cleanup old ticks
		a.cleanupOldTicks(symbol, now)
	}
	return result
}

// Simplified cleanup - keep only ticks within max interval
func (a *KlineAggregator) cleanupOldTicks(symbol string, now int64) {
	oldestValid := now - a.maxIntervalMs
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
