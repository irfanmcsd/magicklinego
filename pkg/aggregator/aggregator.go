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
	lock          sync.Mutex
	Debug         bool
	Logger        *logrus.Logger
	maxIntervalMs int64
}

func NewKlineAggregator(logger *logrus.Logger, debugMode bool) *KlineAggregator {
	intervalToMs := map[string]int64{
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

	var maxMs int64
	for _, ms := range intervalToMs {
		if ms > maxMs {
			maxMs = ms
		}
	}

	return &KlineAggregator{
		tickBuffer:    make(map[string][]TickData),
		Logger:        logger,
		Debug:         debugMode,
		intervalToMs:  intervalToMs,
		maxIntervalMs: maxMs, // Store the calculated max duration
	}
}

/*func NewKlineAggregator(logger *logrus.Logger, debugMode bool) *KlineAggregator {
	return &KlineAggregator{
		tickBuffer: make(map[string][]TickData),
		Logger:     logger,
		Debug:      debugMode,
		intervalToMs: map[string]int64{
			"1m": 60_000,
			"3m": 3 * 60_000,
			"5m": 5 * 60_000,
		},
	}
}*/

/* "5m":  5 * 60_000,
"15m": 15 * 60_000,
"30m": 30 * 60_000,
"1h":  60 * 60_000,
"2h":  2 * 60 * 60_000,
"4h":  4 * 60 * 60_000,
"12h": 12 * 60 * 60_000,
"1d":  24 * 60 * 60_000,
"2d":  2 * 24 * 60 * 60_000,
"3d":  3 * 24 * 60 * 60_000,*/

func (a *KlineAggregator) ExtractOhlc(intervals ...string) []models.SymbolKlineData {
	a.lock.Lock()
	defer a.lock.Unlock()

	if len(intervals) == 0 {
		for k := range a.intervalToMs {
			intervals = append(intervals, k)
		}
		if a.Debug {
			a.Logger.Printf("[DEBUG] No intervals provided. Using all available intervals: %v", intervals)
		}
	}

	now := time.Now().UnixMilli()
	var result []models.SymbolKlineData

	for symbol, ticks := range a.tickBuffer {
		if len(ticks) < 2 {
			if a.Debug {
				a.Logger.Printf("[DEBUG] Skipping %s, not enough ticks (%d)", symbol, len(ticks))
			}
			continue
		}

		for _, interval := range intervals {
			intervalMs, ok := a.intervalToMs[interval]
			if !ok {
				if a.Debug {
					a.Logger.Printf("[DEBUG] Invalid interval: %s", interval)
				}
				continue
			}

			cutoffTime := now - (now % intervalMs)
			grouped := a.groupTicks(ticks, intervalMs, cutoffTime)

			for key, group := range grouped {
				kline := a.buildKline(symbol, interval, group)
				kline.OpenTime = key * intervalMs
				result = append(result, kline)
				if a.Debug {
					a.Logger.Printf("[DEBUG] OHLC generated for %s [%s]: groupKey=%d, ticks=%d, OHLC=%.2f/%.2f/%.2f/%.2f",
						symbol, interval, key, len(group), kline.Open, kline.High, kline.Low, kline.Close)
				}
			}
		}

		//a.cleanupOldTicks(symbol, ticks, intervals, now)
	}

	// ✅ ADD the new, safer cleanup call here, after all processing is done
	a.cleanupAllOldTicks(now)

	if a.Debug {
		a.Logger.Printf("[DEBUG] Total OHLCs extracted: %d", len(result))
	}

	return result
}

func (a *KlineAggregator) groupTicks(ticks []TickData, intervalMs, cutoffTime int64) map[int64][]TickData {
	groupMap := make(map[int64][]TickData)
	for _, tick := range ticks {
		if tick.Time < cutoffTime {
			key := tick.Time / intervalMs
			groupMap[key] = append(groupMap[key], tick)
		}
	}
	return groupMap
}

func (a *KlineAggregator) buildKline(symbol, interval string, group []TickData) models.SymbolKlineData {
	sort.Slice(group, func(i, j int) bool {
		return group[i].Time < group[j].Time
	})

	open := group[0].Price
	close := group[len(group)-1].Price
	high := open
	low := open
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
		Symbol:   symbol,
		Interval: interval,
		Open:     open,
		Close:    close,
		High:     high,
		Low:      low,
		//OpenTime:   group[0].Time,
		Volume:     volumeSum,
		TradeCount: int64(len(group)),
	}
}

func (a *KlineAggregator) cleanupAllOldTicks(now int64) {
	// Keep ticks that are newer than two of the longest interval periods
	cutoff := now - (2 * a.maxIntervalMs)

	for symbol, ticks := range a.tickBuffer {
		var filtered []TickData
		for _, tick := range ticks {
			if tick.Time >= cutoff {
				filtered = append(filtered, tick)
			}
		}
		a.tickBuffer[symbol] = filtered
	}
}

func (a *KlineAggregator) cleanupOldTicks(symbol string, ticks []TickData, intervals []string, now int64) {
	var validCutoffs []int64
	for _, interval := range intervals {
		if ms, ok := a.intervalToMs[interval]; ok {
			validCutoffs = append(validCutoffs, now-(now%ms))
		}
	}

	oldestValid := now
	for _, cutoff := range validCutoffs {
		if cutoff < oldestValid {
			oldestValid = cutoff
		}
	}

	var filtered []TickData
	for _, tick := range ticks {
		if tick.Time >= oldestValid {
			filtered = append(filtered, tick)
		}
	}
	a.tickBuffer[symbol] = filtered
}
