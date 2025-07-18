package db

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/models"
)

var GormDB *gorm.DB

func InitDB() {
	var err error
	conn := config.Settings.Database.ConnectionString
	GormDB, err = gorm.Open(sqlite.Open(conn), &gorm.Config{})
	//GormDB, err = gorm.Open(sqlite.Open("./data.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to initialize GORM DB: %v", err)
	}

	sqlDB, err := GormDB.DB()
	if err != nil {
		log.Fatalf("Failed to get sql.DB from GORM: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("DB connection ping failed: %v", err)
	}

	log.Println("✅ SQLite (glebarez) connected.")
}

func AutoMigrate() error {
	return GormDB.AutoMigrate(
		&models.SymbolKlineData{},
	)
}
