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

type AppSettings struct {
	Exchange   string   `yaml:"exchange"`
	ApiKey     string   `yaml:"apiKey"`
	SecretKey  string   `yaml:"secretKey"`
	PassPhrase string   `yaml:"passPhrase"`
	Symbols    []string `yaml:"symbol"`

	Database struct {
		Provider         string `yaml:"provider"`
		ConnectionString string `yaml:"connectionString"`
	} `yaml:"database"`
}

var Settings AppSettings
