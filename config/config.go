package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	err = yaml.Unmarshal(data, &Settings)
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	log.Println("✅ App settings loaded from", path)
}

type AggregatorSettings struct {
	EnableJitter     bool `yaml:"EnableJitter"`
	JitterMaxMillis  int  `yaml:"JitterMaxMillis"`
	EnableBatchStats bool `yaml:"EnableBatchStats"`
	PersistRawTicks  struct {
		Enabled bool   `yaml:"Enabled"`
		Output  string `yaml:"Output"` // "redis", "kafka"
	} `yaml:"PersistRawTicks"`
}

type AppSettings struct {
	Exchange           string             `yaml:"exchange"`
	Instance           string             `mapstructure:"instance"`
	RefreshSeconds     int                `mapstructure:"RefreshSeconds"`
	Symbols            []string           `yaml:"symbol"`
	BlacklistedSymbols []string           `yaml:"blacklisted_symbols"`
	Aggregator         AggregatorSettings `yaml:"Aggregator"`
	Streaming          StreamingConfig    `yaml:"Streaming"`
	Debug              bool               `yaml:"Debug"`

	Database struct {
		Provider         string `yaml:"provider"`
		ConnectionString string `yaml:"connectionString"`
	} `yaml:"database"`
}

type StreamingConfig struct {
	Enabled  bool   `yaml:"Enabled"`
	Provider string `yaml:"Provider"`

	Redis struct {
		Address  string `yaml:"Address"`
		Stream   string `yaml:"Stream"`
		Password string `yaml:"Password"` // Optional
		DB       int    `yaml:"DB"`       // Optional, default is 0
	} `yaml:"Redis"`

	Kafka struct {
		Brokers []string `yaml:"Brokers"`
		Topic   string   `yaml:"Topic"`
	} `yaml:"Kafka"`
}

var Settings AppSettings
