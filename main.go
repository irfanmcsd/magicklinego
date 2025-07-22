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
	// 🔒 Panic protection
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🔥 Panic recovered: %v\n", r)
		}
	}()

	// 🧾 Initialize logger
	loggerResult, _ := config.InitLogger(true)
	log := loggerResult.Logger

	log.Info("📈 App started")
	log.Debug("🔍 Debug info loaded")

	// ⚙️ Load configuration
	config.LoadConfig("appsettings.yaml")
	log.Info("⚙️ Configuration loaded")

	// 🗃️ Initialize DB
	db.InitDB(log)
	log.Info("🗃️ Database initialized")

	// 🧱 Run DB migrations
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("❌ AutoMigrate failed: %v", err)
	}
	log.Info("✅ Auto-migration complete")

	exchange := config.Settings.Exchange
	symbols := config.Settings.Symbols

	// 🔁 Setup symbol rotator
	batchSize := 50
	rotator := aggregator.NewSymbolRotator(symbols, batchSize)

	// 📊 Initialize Kline aggregator
	kAgg := aggregator.NewKlineAggregator()

	// ⏱️ Setup ticker
	refreshInterval := 5 * time.Second
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	// 🛑 Handle shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.WithField("exchange", exchange).Info("⏳ Starting periodic fetch loop")

	// ✅ Validate and initialize streaming
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
			// 🌪️ Random jitter (up to 500ms)
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			// ♻️ Get next batch of symbols
			batch := rotator.NextBatch()
			if len(batch) == 0 {
				log.Warn("⚠️ No symbols in batch to process")
				continue
			}

			// 📥 Fetch tickers
			tickers, err := exchanges.CoreFuturesAllTickers(exchange)
			if err != nil {
				log.Errorf("❌ Failed to fetch tickers: %v", err)
				continue
			}

			// 🔍 Filter tickers based on batch
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

			// ➕ Feed into aggregator + optional streaming
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

			// ⏱️ Determine flush intervals
			now := time.Now().UTC().Truncate(time.Second)
			flushNow := uniqueStrings(getFlushIntervals(now))

			// 📅 Extract OHLCs
			klineData := kAgg.ExtractOhlc(flushNow...)
			if len(klineData) > 0 {
				log.Infof("📊 Extracted %d OHLC records", len(klineData))

				if err := db.SaveKlines(klineData, log); err != nil {
					log.Errorf("❌ Failed to save klines: %v", err)
				} else {
					log.Infof("✅ Saved %d OHLC entries to DB", len(klineData))
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

// getFlushIntervals returns a list of intervals that need flushing at the current time
func getFlushIntervals(now time.Time) []string {
	var flush []string

	flush = append(flush, "1m", "3m")

	if now.Minute()%5 == 0 {
		flush = append(flush, "5m")
	}
	if now.Minute()%15 == 0 {
		flush = append(flush, "15m")
	}
	if now.Minute()%30 == 0 {
		flush = append(flush, "30m")
	}
	if now.Minute() == 0 {
		flush = append(flush, "1h")
		if now.Hour()%2 == 0 {
			flush = append(flush, "2h")
		}
		if now.Hour()%4 == 0 {
			flush = append(flush, "4h")
		}
		if now.Hour()%12 == 0 {
			flush = append(flush, "12h")
		}
	}
	if now.Hour() == 0 {
		flush = append(flush, "1d")
		if now.Day()%2 == 1 {
			flush = append(flush, "2d")
		}
		if now.Day()%3 == 1 {
			flush = append(flush, "3d")
		}
	}

	return flush
}

// uniqueStrings returns a deduplicated slice
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, val := range input {
		if !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	return result
}
