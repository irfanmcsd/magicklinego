package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"

	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/pkg/exchanges"
)

type TickerInfo struct {
	Symbol             string `json:"symbol"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice          string `json:"lastPrice"`
	Vol24h             string `json:"volume"`
	Timestamp          int64  `json:"timestamp"` // optional: for tracking freshness
}

func PushTickToStream(t *exchanges.TickerInfo, cfg config.StreamingConfig, log *logrus.Logger) {
	if !cfg.Enabled {
		log.Debug("⏩ Streaming disabled")
		return
	}

	// Prepare JSON payload
	payload, err := json.Marshal(t)
	if err != nil {
		log.WithError(err).Error("❌ Failed to marshal tick")
		return
	}

	switch cfg.Provider {
	case "redis":
		pushToRedis(t, cfg.Redis, payload, log)
	case "kafka":
		pushToKafka(cfg.Kafka, payload, log)
	default:
		log.Warnf("⚠️ Unknown streaming provider: %s", cfg.Provider)
	}
}

func pushToRedis(t *exchanges.TickerInfo, cfg struct {
	Address string `yaml:"Address"`
	Stream  string `yaml:"Stream"`
}, payload []byte, log *logrus.Logger) {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Address,
	})

	streamKey := cfg.Stream
	entry := map[string]interface{}{
		"symbol":  t.Symbol,
		"payload": payload,
		"ts":      time.Now().UnixMilli(),
	}

	if err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: entry,
	}).Err(); err != nil {
		log.WithError(err).WithField("symbol", t.Symbol).Error("❌ Failed to push tick to Redis")
	} else {
		log.WithField("symbol", t.Symbol).Debug("📤 Tick sent to Redis stream")
	}
}

func pushToKafka(cfg struct {
	Brokers []string `yaml:"Brokers"`
	Topic   string   `yaml:"Topic"`
}, payload []byte, log *logrus.Logger) {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", time.Now().UnixNano())),
		Value: payload,
	}

	if err := writer.WriteMessages(context.Background(), msg); err != nil {
		log.WithError(err).Error("❌ Failed to write to Kafka")
	} else {
		log.Debug("📤 Tick sent to Kafka")
	}
	_ = writer.Close()
}
