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
	Exchange   string             `yaml:"exchange"`
	ApiKey     string             `yaml:"apiKey"`
	SecretKey  string             `yaml:"secretKey"`
	PassPhrase string             `yaml:"passPhrase"`
	Symbols    []string           `yaml:"symbol"`
	Aggregator AggregatorSettings `yaml:"Aggregator"`
	Streaming  StreamingConfig    `yaml:"Streaming"`

	Database struct {
		Provider         string `yaml:"provider"`
		ConnectionString string `yaml:"connectionString"`
	} `yaml:"database"`
}

type StreamingConfig struct {
	Enabled  bool   `yaml:"Enabled"`
	Provider string `yaml:"Provider"`

	Redis struct {
		Address string `yaml:"Address"`
		Stream  string `yaml:"Stream"`
	} `yaml:"Redis"`

	Kafka struct {
		Brokers []string `yaml:"Brokers"`
		Topic   string   `yaml:"Topic"`
	} `yaml:"Kafka"`
}

var Settings AppSettings
