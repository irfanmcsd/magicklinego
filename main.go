// kline-scanner-go/main.go
package main

import (
	"github.com/sirupsen/logrus"
	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/pkg/db"
)

func main() {
	loggerResult, _ := config.InitLogger(true)
	log := loggerResult.Logger

	log.Info("📈 App started")
	log.Debug("🔍 Debug info loaded")

	log.WithFields(logrus.Fields{
		"symbol": "BTCUSDT",
		"status": "scanning",
	}).Info("🛰️ Scanner update")

	// Load application config
	config.LoadConfig("appsettings.yaml")
	log.Info("⚙️ Configuration loaded")

	// Initialize DB
	db.InitDB(loggerResult.Logger)
	log.Info("🗃️ Database initialized")

	// Run DB migrations
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("❌ AutoMigrate failed: %v", err)
	}

	log.Info("✅ Auto-migration complete")
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
