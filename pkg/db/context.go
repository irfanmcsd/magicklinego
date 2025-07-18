package db

import (
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/models"
)

var GormDB *gorm.DB

func InitDB(log *logrus.Logger) {
	dbFile := config.Settings.Database.ConnectionString

	// Check if DB file exists, log only (do NOT create)
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		log.Warnf("⚠️  Database file '%s' does not exist. It will be created on first use by SQLite.", dbFile)
	} else if err != nil {
		log.Fatalf("❌ Failed to stat database file: %v", err)
	}

	// GORM will NOT recreate existing files, only open them
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Failed to open GORM SQLite DB: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to extract sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ DB ping failed: %v", err)
	}

	GormDB = db
	log.Infof("✅ SQLite connected using file: %s", dbFile)
}

func AutoMigrate() error {
	// Safe — this will NOT drop existing tables or data
	return GormDB.AutoMigrate(
		&models.SymbolKlineData{},
	)
}

func SaveKlines(data []models.SymbolKlineData, log *logrus.Logger) error {
	if len(data) == 0 {
		log.Info("📭 No klines to insert")
		return nil
	}

	result := GormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "symbol"},
			{Name: "interval"},
			{Name: "open_time"},
		},
		DoNothing: true, // skip duplicates
	}).CreateInBatches(data, 100)

	if result.Error != nil {
		return fmt.Errorf("insert failed: %w", result.Error)
	}

	log.WithFields(logrus.Fields{
		"attempted": len(data),
		"inserted":  result.RowsAffected,
	}).Info("✅ Saved klines")

	return nil
}
