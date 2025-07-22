package main

import (
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

	// 📊 Initialize kline aggregator
	kAgg := aggregator.NewKlineAggregator()

	// ⏱️ Ticker every 5 seconds
	refreshInterval := 5 * time.Second
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	// 🛑 Handle shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Infof("⏳ Starting periodic fetch loop for exchange: %s", exchange)

	// ✅ Validate streaming config if enabled
	streamCfg := config.Settings.Streaming
	if streamCfg.Enabled {
		if streamCfg.Provider != "redis" && streamCfg.Provider != "kafka" {
			log.Fatalf("❌ Invalid streaming provider: %s", streamCfg.Provider)
		}
		if streamCfg.Provider == "redis" && streamCfg.Redis.Address == "" {
			log.Fatal("❌ Redis streaming enabled but Redis.Address is empty")
		}
		if streamCfg.Provider == "kafka" && len(streamCfg.Kafka.Brokers) == 0 {
			log.Fatal("❌ Kafka streaming enabled but no Kafka.Brokers specified")
		}
		log.Infof("📤 Streaming enabled using provider: %s", streamCfg.Provider)
	}

	global.InitStreamingClients(streamCfg)
	defer global.ShutdownStreamingClients()

loop:
	for {
		select {
		case <-ticker.C:
			// 🌪️ Apply random jitter (up to 500ms)
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(jitter)

			// ♻️ Rotate next batch
			batch := rotator.NextBatch()
			if len(batch) == 0 {
				log.Warn("⚠️ No symbols in batch to process")
				continue
			}

			// 📥 Fetch all tickers
			tickers, err := exchanges.CoreFuturesAllTickers(exchange)
			if err != nil {
				log.Errorf("❌ Failed to fetch tickers: %v", err)
				continue
			}
			log.Infof("📥 Fetched %d tickers from %s", len(tickers), exchange)

			// 🔍 Filter tickers based on batch
			symbolSet := make(map[string]bool)
			for _, sym := range batch {
				symbolSet[strings.ToUpper(sym)] = true
			}

			filtered := []*exchanges.TickerInfo{}
			for _, t := range tickers {
				if symbolSet[strings.ToUpper(t.Symbol)] {
					filtered = append(filtered, t)
				}
			}
			tickers = filtered
			log.Infof("🔄 Rotated batch matched: %d symbols", len(tickers))

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

				// 📤 Push raw tick to stream (via goroutine if streaming is IO-bound)
				if streamCfg.Enabled {
					tick := ConvertToAggregatorTicker(t)
					go aggregator.PushTickToStream(tick, streamCfg, log)
					// OR use synchronous version if needed:
					// aggregator.PushTickToStream(t, streamCfg, log)
				}
			}

			// 🕰️ Determine flush intervals
			now := time.Now().UTC().Truncate(time.Second)
			flushNow := uniqueStrings(getFlushIntervals(now))

			// 📅 Extract OHLCs
			klineData := kAgg.ExtractOhlc(flushNow...)
			if len(klineData) > 0 {
				log.Infof("📊 Extracted %d OHLC records", len(klineData))

				// 💾 Save to DB
				err := db.SaveKlines(klineData, log)
				if err != nil {
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
