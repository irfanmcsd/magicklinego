// streaming/clients.go
package global

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"scanner.magictradebot.com/config"
)

var (
	RedisClient *redis.Client
	KafkaWriter *kafka.Writer
)

// InitStreamingClients initializes Redis or Kafka clients based on the config.
func InitStreamingClients(cfg config.StreamingConfig) {
	if cfg.Provider == "redis" && cfg.Redis.Address != "" {
		RedisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Address,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		// Optional: test connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := RedisClient.Ping(ctx).Err(); err != nil {
			panic("❌ Failed to connect to Redis: " + err.Error())
		}
	}

	if cfg.Provider == "kafka" && len(cfg.Kafka.Brokers) > 0 {
		KafkaWriter = &kafka.Writer{
			Addr:         kafka.TCP(cfg.Kafka.Brokers...),
			Topic:        cfg.Kafka.Topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		}
	}
}

// ShutdownStreamingClients gracefully closes any initialized clients.
func ShutdownStreamingClients() {
	if RedisClient != nil {
		_ = RedisClient.Close()
	}
	if KafkaWriter != nil {
		_ = KafkaWriter.Close()
	}
}

func ValidateStreamingConfig(cfg config.StreamingConfig, log *logrus.Logger) error {
	if !cfg.Enabled {
		log.Info("🔇 Streaming is disabled.")
		return nil
	}

	switch cfg.Provider {
	case "redis":
		if cfg.Redis.Address == "" || cfg.Redis.Stream == "" {
			return fmt.Errorf("❌ Redis configuration is incomplete")
		}
	case "kafka":
		if len(cfg.Kafka.Brokers) == 0 || cfg.Kafka.Topic == "" {
			return fmt.Errorf("❌ Kafka configuration is incomplete")
		}
	default:
		return fmt.Errorf("❌ Unknown streaming provider: %s", cfg.Provider)
	}

	return nil
}
