package aggregator

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"scanner.magictradebot.com/config"
	"scanner.magictradebot.com/pkg/global"
	"scanner.magictradebot.com/pkg/utils"
)

type TickerInfo struct {
	Symbol             string `json:"symbol"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice          string `json:"lastPrice"`
	Vol24h             string `json:"volume"`
	Timestamp          int64  `json:"timestamp"` // optional: for tracking freshness
}

func PushTickToStream(t TickerInfo, cfg config.StreamingConfig, log *logrus.Logger) {
	if !cfg.Enabled {
		log.Debug("⏩ Streaming disabled")
		return
	}

	payload, err := json.Marshal(t)
	if err != nil {
		log.WithError(err).Error("❌ Failed to marshal tick")
		return
	}

	switch cfg.Provider {
	case "redis":
		entry := utils.CreateRedisStreamEntry(t.Symbol, payload)
		ctx := context.Background()
		err := global.RedisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: cfg.Redis.Stream,
			Values: entry,
		}).Err()
		if err != nil {
			log.WithError(err).WithField("symbol", t.Symbol).Error("❌ Redis stream error")
		} else {
			log.WithField("symbol", t.Symbol).Debug("📤 Tick sent to Redis stream")
		}

	case "kafka":
		err := utils.WriteKafkaMessage(global.KafkaWriter, payload)
		if err != nil {
			log.WithError(err).Error("❌ Kafka send error")
		} else {
			log.Debug("📤 Tick sent to Kafka")
		}

	default:
		log.Warnf("⚠️ Unknown streaming provider: %s", cfg.Provider)
	}
}
