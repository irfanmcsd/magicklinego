// kline-scanner-go/main.go
package main

import (
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
)

func main() {
	// Initialize logger
	loggerResult, _ := config.InitLogger(true)
	log := loggerResult.Logger

	log.Info("📈 App started")
	log.Debug("🔍 Debug info loaded")

	log.WithFields(logrus.Fields{
		"symbol": "BTCUSDT",
		"status": "scanning",
	}).Info("🛰️ Scanner update")

	// Load configuration
	config.LoadConfig("appsettings.yaml")
	log.Info("⚙️ Configuration loaded")

	// Initialize DB
	db.InitDB(log)
	log.Info("🗃️ Database initialized")

	// Run DB migrations
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("❌ AutoMigrate failed: %v", err)
	}
	log.Info("✅ Auto-migration complete")

	exchange := config.Settings.Exchange
	symbols := config.Settings.Symbols

	// Prepare filters if symbols provided
	symbolSet := map[string]bool{}
	if len(symbols) > 0 {
		for _, sym := range symbols {
			symbolSet[strings.ToUpper(sym)] = true
		}
	}

	// Setup aggregator
	kAgg := aggregator.NewKlineAggregator()

	// Ticker for refresh
	refreshInterval := 5 * time.Second
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	// Shutdown hook
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Infof("⏳ Starting periodic fetch loop for exchange: %s", exchange)

loop:
	for {
		select {
		case <-ticker.C:
			tickers, err := exchanges.CoreFuturesAllTickers(exchange)
			if err != nil {
				log.Errorf("❌ Failed to fetch tickers: %v", err)
				continue
			}

			log.Infof("📥 Fetched %d tickers from %s", len(tickers), exchange)

			// Apply symbol filter
			if len(symbolSet) > 0 {
				filtered := []*exchanges.TickerInfo{}
				for _, t := range tickers {
					if symbolSet[strings.ToUpper(t.Symbol)] {
						filtered = append(filtered, t)
					}
				}
				tickers = filtered
				log.Infof("🔎 Filtered to %d matching symbols", len(tickers))
			}

			// Add tick data to aggregator
			for _, t := range tickers {
				price, err := strconv.ParseFloat(t.LastPrice, 64)
				if err != nil {
					log.Printf("❌ Failed to parse price for %s: %v", t.Symbol, err)
					continue
				}

				volume, err := strconv.ParseFloat(t.Vol24h, 64)
				if err != nil {
					log.Printf("❌ Failed to parse volume for %s: %v", t.Symbol, err)
					continue
				}

				kAgg.AddPrice(t.Symbol, price, volume)
			}

			// Determine which intervals to flush
			now := time.Now().UTC()
			flushNow := []string{"1m", "3m"}
			if now.Minute()%5 == 0 {
				flushNow = append(flushNow, "5m")
			}
			if now.Minute()%15 == 0 {
				flushNow = append(flushNow, "15m")
			}
			if now.Minute()%30 == 0 {
				flushNow = append(flushNow, "30m")
			}
			if now.Minute() == 0 {
				flushNow = append(flushNow, "1h")
				if now.Hour()%2 == 0 {
					flushNow = append(flushNow, "2h")
				}
				if now.Hour()%4 == 0 {
					flushNow = append(flushNow, "4h")
				}
				if now.Hour()%12 == 0 {
					flushNow = append(flushNow, "12h")
				}
			}
			if now.Hour() == 0 {
				flushNow = append(flushNow, "1d")
				if now.Day()%2 == 1 {
					flushNow = append(flushNow, "2d")
				}
				if now.Day()%3 == 1 {
					flushNow = append(flushNow, "3d")
				}
			}

			// Remove duplicates
			intervalSet := map[string]bool{}
			for _, i := range flushNow {
				intervalSet[i] = true
			}
			finalFlush := []string{}
			for k := range intervalSet {
				finalFlush = append(finalFlush, k)
			}

			// Extract OHLCs
			klineData := kAgg.ExtractOhlc(finalFlush...)
			if len(klineData) > 0 {
				log.Infof("📊 Extracted %d OHLC records", len(klineData))
				err := db.SaveKlines(klineData)
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

/*
func main() {

	loggerResult, _ := config.InitLogger(true)
	log := loggerResult.Logger

	log.Info("📈 App started")
	log.Debug("🔍 Debug info loaded")
	log.WithFields(logrus.Fields{
		"symbol": "BTCUSDT",
		"status": "scanning",
	}).Info("Scanner update")

	config.LoadConfig("appsettings.yaml")

	db.InitDB()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}

	log.Println("✅ Auto-migration complete")

	// 1. Initialize Redis & DB
	db := storage.NewPostgres()
	redis := storage.NewRedis()

	// 2. Load all supported symbols (from Binance for now)
	symbols := exchange.GetSupportedSymbols("binance") // TODO: extend for okx, bybit, etc.
	if len(symbols) == 0 {
		log.Fatal("No symbols found from exchange")
	}

	// 3. Group symbols (5–10 per batch)
	groups := scheduler.CreateSymbolGroups(symbols, 10)

	// 4. Schedule each group every 5s, round-robin
	scheduler.Run(ctx, groups, func(group []string) {
		for _, symbol := range group {
			kline := exchange.FetchLatestKline(symbol)
			if kline != nil {
				aggregator.SaveKline(redis, db, symbol, kline)
			}
		}
	})

	select {} // block forever
}
*/
