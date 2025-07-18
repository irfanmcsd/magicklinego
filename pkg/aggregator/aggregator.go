package aggregator

import (
	"sort"
	"sync"
	"time"

	"scanner.magictradebot.com/models"
)

type Aggregator struct {
	mu        sync.Mutex
	priceData map[string][]PricePoint
}

type PricePoint struct {
	Time   time.Time
	Price  float64
	Volume float64
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		priceData: make(map[string][]PricePoint),
	}
}

func (a *Aggregator) AddPrice(symbol string, price, volume float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	point := PricePoint{
		Time:   time.Now().UTC().Truncate(time.Minute),
		Price:  price,
		Volume: volume,
	}
	a.priceData[symbol] = append(a.priceData[symbol], point)
}

type TickData struct {
	Price  float64
	Time   int64
	Volume float64
}

type KlineAggregator struct {
	tickBuffer   map[string][]TickData
	intervalToMs map[string]int64
	lock         sync.Mutex
}

func NewKlineAggregator() *KlineAggregator {
	return &KlineAggregator{
		tickBuffer: make(map[string][]TickData),
		intervalToMs: map[string]int64{
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
		},
	}
}

func (a *KlineAggregator) AddPrice(symbol string, price float64, volume float64) {
	a.lock.Lock()
	defer a.lock.Unlock()

	now := time.Now().UnixMilli()
	a.tickBuffer[symbol] = append(a.tickBuffer[symbol], TickData{
		Price:  price,
		Time:   now,
		Volume: volume,
	})
}

func (a *KlineAggregator) ExtractOhlc(intervals ...string) []models.SymbolKlineData {
	a.lock.Lock()
	defer a.lock.Unlock()

	if len(intervals) == 0 {
		for k := range a.intervalToMs {
			intervals = append(intervals, k)
		}
	}

	now := time.Now().UnixMilli()
	var result []models.SymbolKlineData

	for symbol, ticks := range a.tickBuffer {
		if len(ticks) < 2 {
			continue
		}

		for _, interval := range intervals {
			intervalMs, ok := a.intervalToMs[interval]
			if !ok {
				continue
			}

			cutoffTime := now - (now % intervalMs)
			groupMap := make(map[int64][]TickData)

			for _, tick := range ticks {
				if tick.Time < cutoffTime {
					key := tick.Time / intervalMs
					groupMap[key] = append(groupMap[key], tick)
				}
			}

			for _, group := range groupMap {
				sort.Slice(group, func(i, j int) bool {
					return group[i].Time < group[j].Time
				})

				var (
					open      = group[0].Price
					close     = group[len(group)-1].Price
					high      = group[0].Price
					low       = group[0].Price
					volumeSum float64
				)

				for _, tick := range group {
					if tick.Price > high {
						high = tick.Price
					}
					if tick.Price < low {
						low = tick.Price
					}
					volumeSum += tick.Volume
				}

				result = append(result, models.SymbolKlineData{
					Symbol:     symbol,
					Interval:   interval,
					Open:       open,
					Close:      close,
					High:       high,
					Low:        low,
					OpenTime:   group[0].Time,
					Volume:     volumeSum,
					TradeCount: int64(len(group)),
				})
			}
		}

		// Clean up old ticks
		var validCutoffs []int64
		for _, interval := range intervals {
			if ms, ok := a.intervalToMs[interval]; ok {
				validCutoffs = append(validCutoffs, now-(now%ms))
			}
		}
		var oldestValid int64 = now
		for _, cutoff := range validCutoffs {
			if cutoff < oldestValid {
				oldestValid = cutoff
			}
		}

		// Retain only recent ticks
		var filtered []TickData
		for _, tick := range ticks {
			if tick.Time >= oldestValid {
				filtered = append(filtered, tick)
			}
		}
		a.tickBuffer[symbol] = filtered
	}

	return result
}

type OHLC struct {
	Symbol    string
	Interval  string
	StartTime time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

func (a *Aggregator) ExtractOhlc(intervals []string) []OHLC {
	a.mu.Lock()
	defer a.mu.Unlock()

	var results []OHLC

	for symbol, points := range a.priceData {
		for _, interval := range intervals {
			duration, ok := ParseIntervalToDuration(interval)
			if !ok {
				continue
			}
			grouped := make(map[time.Time][]PricePoint)

			for _, p := range points {
				bucket := p.Time.Truncate(duration)
				grouped[bucket] = append(grouped[bucket], p)
			}

			for bucket, group := range grouped {
				if len(group) == 0 {
					continue
				}
				open := group[0].Price
				close := group[len(group)-1].Price
				high, low := group[0].Price, group[0].Price
				var volume float64

				for _, pt := range group {
					if pt.Price > high {
						high = pt.Price
					}
					if pt.Price < low {
						low = pt.Price
					}
					volume += pt.Volume
				}

				results = append(results, OHLC{
					Symbol:    symbol,
					Interval:  interval,
					StartTime: bucket,
					Open:      open,
					High:      high,
					Low:       low,
					Close:     close,
					Volume:    volume,
				})
			}
		}
	}

	// Clear points to avoid duplicate aggregation
	a.priceData = make(map[string][]PricePoint)
	return results
}

func ParseIntervalToDuration(interval string) (time.Duration, bool) {
	switch interval {
	case "1m":
		return time.Minute, true
	case "3m":
		return 3 * time.Minute, true
	case "5m":
		return 5 * time.Minute, true
	case "15m":
		return 15 * time.Minute, true
	case "30m":
		return 30 * time.Minute, true
	case "1h":
		return time.Hour, true
	case "2h":
		return 2 * time.Hour, true
	case "4h":
		return 4 * time.Hour, true
	case "12h":
		return 12 * time.Hour, true
	case "1d":
		return 24 * time.Hour, true
	case "2d":
		return 48 * time.Hour, true
	case "3d":
		return 72 * time.Hour, true
	default:
		return 0, false
	}
}
