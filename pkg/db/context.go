package db

import (
	"fmt"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	_ "github.com/lib/pq" // <-- Add this for PostgreSQL
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/models"
)

var GormDB *gorm.DB

func InitDB(log *logrus.Logger) {
	provider := config.Settings.Database.Provider
	conn := config.Settings.Database.ConnectionString

	var db *gorm.DB
	var err error

	switch provider {
	case "sqlite":
		if _, err := os.Stat(conn); os.IsNotExist(err) {
			log.Warnf("⚠️  SQLite DB file '%s' does not exist. Will be created on first write.", conn)
		}
		db, err = gorm.Open(sqlite.Open(conn), &gorm.Config{})
		if err != nil {
			log.Fatalf("❌ Failed to open SQLite DB: %v", err)
		}
		log.Infof("✅ SQLite connected: %s", conn)

	case "postgresql":
		db, err = gorm.Open(postgres.Open(conn), &gorm.Config{})
		if err != nil {
			log.Fatalf("❌ Failed to connect to PostgreSQL: %v", err)
		}
		log.Infof("✅ PostgreSQL connected")

	default:
		log.Fatalf("❌ Unknown DB provider: %s", provider)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to extract sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ DB ping failed: %v", err)
	}

	GormDB = db
}

func AutoMigrate() error {
	// Safe — this will NOT drop existing tables or data
	return GormDB.AutoMigrate(
		&models.SymbolKlineData{},
	)
}

func SaveKlines(data []models.SymbolKlineData, instance string, log *logrus.Logger) error {
	if len(data) == 0 {
		log.WithField("instance", instance).Info("📭 No klines to insert")
		return nil
	}

	// Set instance and clean symbol
	for i := range data {
		data[i].Instance = instance
		data[i].Symbol = cleanSymbol(data[i].Symbol) // <-- Apply cleaner here
	}

	// Group data by symbol and interval for detailed logging
	type logKey struct {
		Symbol   string
		Interval string
	}

	logSummary := make(map[logKey]int)

	for _, k := range data {
		key := logKey{Symbol: k.Symbol, Interval: k.Interval}
		logSummary[key]++
	}

	result := GormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "symbol"},
			{Name: "interval"},
			{Name: "open_time"},
		},
		DoNothing: true,
	}).CreateInBatches(data, 100)

	if result.Error != nil {
		return fmt.Errorf("insert failed: %w", result.Error)
	}

	for key, count := range logSummary {
		log.WithFields(logrus.Fields{
			"instance":  instance,
			"symbol":    key.Symbol,
			"interval":  key.Interval,
			"attempted": count,
			"inserted":  result.RowsAffected, // optional: total inserted
		}).Infof("✅ Saved klines for %s [%s]", key.Symbol, key.Interval)
	}

	log.WithFields(logrus.Fields{
		"instance":  instance,
		"attempted": len(data),
		"inserted":  result.RowsAffected,
	}).Info("✅ Saved all OHLC entries to DB")

	return nil
}

func cleanSymbol(raw string) string {
	raw = strings.ToUpper(raw) // Normalize

	// Case 1: Hyphen-based format
	if strings.Contains(raw, "-") {
		suffixes := []string{"-USDT-SWAP", "-USDT", "-USD-SWAP", "-USD", "-PERP", "-FUTURE", "-SWAP"}
		for _, s := range suffixes {
			if strings.HasSuffix(raw, s) {
				return strings.TrimSuffix(raw, s)
			}
		}
		return raw
	}

	// Case 2: Concatenated format (e.g. BTCUSDT, ETHUSD)
	quoteAssets := []string{"USDT", "USD", "BUSD", "USDC", "TUSD", "DAI", "USDT_UMCBL"}
	for _, quote := range quoteAssets {
		if strings.HasSuffix(raw, quote) {
			return strings.TrimSuffix(raw, quote)
		}
	}

	// Default: return raw symbol
	return raw
}
