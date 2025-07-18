// kline-scanner-go/main.go
package main

import (
	"log"

	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/pkg/db"
)

func main() {

	log.Println("📈 Starting Kline Scanner...")

	config.LoadConfig("appsettings.yaml")

	db.InitDB()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}

	log.Println("✅ Auto-migration complete")

	/*// 1. Initialize Redis & DB
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

	select {} // block forever*/
}
