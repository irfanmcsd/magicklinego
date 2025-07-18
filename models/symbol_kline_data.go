// models/symbol_kline_data.go
package models

func (SymbolKlineData) TableName() string {
	return "Dev_SymbolKlineData"
}

type SymbolKlineData struct {
	ID         int64   `gorm:"primaryKey;autoIncrement"`
	Symbol     string  `gorm:"size:50;index:idx_symbol_interval_time"`
	Interval   string  `gorm:"size:10;default:1m;index:idx_symbol_interval_time"`
	Open       float64 `gorm:"type:decimal(18,8)"`
	High       float64 `gorm:"type:decimal(18,8)"`
	Low        float64 `gorm:"type:decimal(18,8)"`
	Close      float64 `gorm:"type:decimal(18,8)"`
	OpenTime   int64   `gorm:"index:idx_symbol_interval_time"`
	Volume     float64
	TradeCount int64
}
