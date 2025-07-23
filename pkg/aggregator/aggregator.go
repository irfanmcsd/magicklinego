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

	// Ensure symbol entry exists
	if _, exists := a.tickBuffer[symbol]; !exists {
		a.tickBuffer[symbol] = make(map[string][]TickData)
	}

	// Add tick to every interval for that symbol
	for interval := range a.intervalToMs {
		intervalTicks := a.tickBuffer[symbol][interval]
		// Check if tick already exists
		exists := false
		for _, t := range intervalTicks {
			if t.Time == truncated {
				exists = true
				break
			}
		}
		if !exists {
			a.tickBuffer[symbol][interval] = append(intervalTicks, tick)
			if a.Debug {
				//a.Logger.Printf("[DEBUG] Added tick to %s [%s]: %.2f", symbol, interval, price)
			}
		}
	}
}

type KlineAggregator struct {
	tickBuffer    map[string]map[string][]TickData // ✅ Changed
	intervalToMs  map[string]int64
	maxIntervalMs int64
	lock          sync.Mutex
	Debug         bool
	Logger        *logrus.Logger
}

func NewKlineAggregator(logger *logrus.Logger, debugMode bool) *KlineAggregator {
	intervals := map[string]int64{
		"1m": 60_000,
	}

	return &KlineAggregator{
		tickBuffer:    map[string]map[string][]TickData{"1m": {}},
		intervalToMs:  intervals,
		maxIntervalMs: 60_000,
		Logger:        logger,
		Debug:         debugMode,
	}
}

/*
func NewKlineAggregator(logger *logrus.Logger, debugMode bool) *KlineAggregator {
	intervals := map[string]int64{
		"1m": 60_000,
	}

	// Find max interval
	var maxIntervalMs int64
	for _, ms := range intervals {
		if ms > maxIntervalMs {
			maxIntervalMs = ms
		}
	}

	return &KlineAggregator{
		tickBuffer:    make(map[string]map[string][]TickData),
		intervalToMs:  intervals,
		maxIntervalMs: maxIntervalMs,
		Logger:        logger,
		Debug:         debugMode,
	}
}*/

func (a *KlineAggregator) ExtractOhlc(intervals ...string) []models.SymbolKlineData {
	a.lock.Lock()
	defer a.lock.Unlock()

	now := time.Now().UnixMilli()
	var result []models.SymbolKlineData

	for symbol, intervalMap := range a.tickBuffer {
		for _, interval := range intervals {
			ticks, exists := intervalMap[interval]
			if !exists || len(ticks) == 0 {
				continue
			}

			sort.Slice(ticks, func(i, j int) bool {
				return ticks[i].Time < ticks[j].Time
			})

			intervalMs, ok := a.intervalToMs[interval]
			if !ok {
				continue
			}

			candleEnd := now - (now % intervalMs)
			candleStart := candleEnd - intervalMs

			startIdx := sort.Search(len(ticks), func(i int) bool {
				return ticks[i].Time >= candleStart
			})

			endIdx := sort.Search(len(ticks), func(i int) bool {
				return ticks[i].Time >= candleEnd
			})

			if startIdx == endIdx {
				continue
			}

			group := ticks[startIdx:endIdx]
			kline := a.buildKline(symbol, interval, group, candleStart)
			result = append(result, kline)

			// Remove used ticks for this interval
			a.tickBuffer[symbol][interval] = ticks[endIdx:]
		}

		// ✅ Clean up old ticks for this symbol
		a.cleanupOldTicks(symbol, now)
	}

	return result
}

func (a *KlineAggregator) cleanupOldTicks(symbol string, now int64) {
	retention := a.maxIntervalMs * 3
	oldestValid := now - retention

	for interval, ticks := range a.tickBuffer[symbol] {
		filtered := []TickData{}
		for _, tick := range ticks {
			if tick.Time >= oldestValid {
				filtered = append(filtered, tick)
			}
		}
		a.tickBuffer[symbol][interval] = filtered

		if a.Debug && len(ticks) != len(filtered) {
			a.Logger.Printf("[DEBUG] Cleaned %d old ticks from %s [%s]", len(ticks)-len(filtered), symbol, interval)
		}
	}

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
