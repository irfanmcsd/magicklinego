package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/pkg/aggregator"
	"scanner.magictradebot.com/pkg/db"
	"scanner.magictradebot.com/pkg/exchanges"
	"scanner.magictradebot.com/pkg/global"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🔥 Panic recovered: %v\n", r)
		}
	}()

	loggerResult, _ := config.InitLogger(true)
	log := loggerResult.Logger

	log.Info("📈 App started")
	log.Debug("🔍 Debug info loaded")

	config.LoadConfig("appsettings.yaml")
	log.Info("⚙️ Configuration loaded")

	db.InitDB(log)
	log.Info("🗃️ Database initialized")

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("❌ AutoMigrate failed: %v", err)
	}
	log.Info("✅ Auto-migration complete")

	exchange := config.Settings.Exchange
	symbols := config.Settings.Symbols

	// ❌ Load blacklisted symbols from config
	invalidSymbols := make(map[string]bool)
	for _, sym := range config.Settings.BlacklistedSymbols {
		invalidSymbols[strings.ToUpper(sym)] = true
	}

	batchSize := 50
	rotator := aggregator.NewSymbolRotator(symbols, batchSize)

	kAgg := aggregator.NewKlineAggregator(log, config.Settings.Debug)

	if config.Settings.RefreshSeconds < 4 {
		config.Settings.RefreshSeconds = 4
	}
	refreshInterval := time.Duration(config.Settings.RefreshSeconds) * time.Second
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.WithField("exchange", exchange).Info("⏳ Starting periodic fetch loop")

	streamCfg := config.Settings.Streaming
	if err := global.ValidateStreamingConfig(streamCfg, log); err != nil {
		log.Fatal(err)
	}

	if streamCfg.Enabled {
		global.InitStreamingClients(streamCfg)
		defer global.ShutdownStreamingClients()
	}

loop:
	for {
		select {
		case <-ticker.C:
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			rawBatch := rotator.NextBatch()
			batch := make([]string, 0, len(rawBatch))
			for _, sym := range rawBatch {
				if !invalidSymbols[strings.ToUpper(sym)] {
					batch = append(batch, sym)
				}
			}

			if len(batch) == 0 {
				log.Warn("⚠️ No valid symbols in batch to process")
				continue
			}

			tickers, err := exchanges.CoreFuturesAllTickers(exchange)
			if err != nil {
				log.Errorf("❌ Failed to fetch tickers: %v", err)
				continue
			}

			symbolSet := make(map[string]bool)
			for _, sym := range batch {
				symbolSet[strings.ToUpper(sym)] = true
			}

			filtered := make([]*exchanges.TickerInfo, 0, len(batch))
			for _, t := range tickers {
				if symbolSet[strings.ToUpper(t.Symbol)] {
					filtered = append(filtered, t)
				}
			}
			tickers = filtered

			log.WithFields(logrus.Fields{
				"fetched":   len(tickers),
				"requested": len(batch),
			}).Info("📥 Batch ticker fetch complete")

			// 🛑 Detect and persist new blacklisted symbols
			foundSymbols := make(map[string]bool)
			for _, t := range tickers {
				foundSymbols[strings.ToUpper(t.Symbol)] = true
			}
			for _, sym := range batch {
				up := strings.ToUpper(sym)
				if !foundSymbols[up] && !invalidSymbols[up] {
					log.WithField("symbol", up).Warn("🚫 Symbol not found in exchange response, blacklisting and saving to config")
					invalidSymbols[up] = true
					config.Settings.BlacklistedSymbols = append(config.Settings.BlacklistedSymbols, up)
					if err := config.SaveConfig("appsettings.yaml"); err != nil {
						log.Errorf("❌ Failed to save updated config: %v", err)
					}
				}
			}

			for _, t := range tickers {
				price, err := strconv.ParseFloat(t.LastPrice, 64)
				if err != nil {
					log.WithFields(logrus.Fields{
						"symbol": t.Symbol,
						"value":  t.LastPrice,
					}).Warnf("❌ Failed to parse price: %v", err)
					continue
				}

				volume, err := strconv.ParseFloat(t.Vol24h, 64)
				if err != nil {
					log.WithFields(logrus.Fields{
						"symbol": t.Symbol,
						"value":  t.Vol24h,
					}).Warnf("❌ Failed to parse volume: %v", err)
					continue
				}

				kAgg.AddPrice(t.Symbol, price, volume)

				if streamCfg.Enabled {
					tick := ConvertToAggregatorTicker(t)
					go aggregator.PushTickToStream(tick, streamCfg, log)
				}
			}

			now := time.Now().UTC().Truncate(time.Second)
			flushNow := getFlushIntervals(now, log)

			if kAgg.Debug {
				kAgg.Logger.Infof("[Debug] Flushing intervals: %v", flushNow)
			}

			if len(flushNow) > 0 {
				klineData := kAgg.ExtractOhlc(flushNow...)
				if len(klineData) > 0 {
					log.Infof("📊 Extracted %d OHLC records", len(klineData))
					if err := db.SaveKlines(klineData, config.Settings.Instance, log); err != nil {
						log.Errorf("❌ Failed to save klines: %v", err)
					} else {
						log.Infof("✅ Saved %d OHLC entries to DB", len(klineData))
					}
				}
			} else {
				if kAgg.Debug {
					kAgg.Logger.Debug("No intervals to flush this cycle")
				}
			}

		case <-stop:
			log.Info("🛑 Shutdown signal received")
			break loop
		}
	}

	log.Info("👋 App shutdown complete")
}

func ConvertToAggregatorTicker(t *exchanges.TickerInfo) aggregator.TickerInfo {
	return aggregator.TickerInfo{
		Symbol:             t.Symbol,
		LastPrice:          t.LastPrice,
		Vol24h:             t.Vol24h,
		PriceChangePercent: t.Change24h,
		Timestamp:          t.Timestamp,
		// Add more fields if needed
	}
}

func getFlushIntervals(now time.Time, log *logrus.Logger) []string {
	aligned := now.Truncate(time.Minute).UnixMilli()

	log.Infof("⏱️ Now: %s (%d)", now.UTC().Format("15:04:05"), aligned)
	log.Infof("📦 Returning intervals: [1m]")

	return []string{"1m"}
}

// uniqueStrings returns a deduplicated slice
/*func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, val := range input {
		if !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	return result
}*/
