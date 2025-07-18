package db

import (
	"database/sql"
	"log"
)

var DB *sql.DB

func InitSQLite(path string) error {
	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Println("✅ SQLite connected at", path)
	return nil
}
