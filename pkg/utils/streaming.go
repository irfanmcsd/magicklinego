package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// CreateRedisStreamEntry creates a Redis stream entry with consistent fields
func CreateRedisStreamEntry(symbol string, payload []byte) map[string]interface{} {
	return map[string]interface{}{
		"symbol":  symbol,
		"payload": payload,
		"ts":      time.Now().UnixMilli(),
	}
}

// WriteKafkaMessage encapsulates Kafka message writing logic
func WriteKafkaMessage(writer *kafka.Writer, payload []byte) error {
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", time.Now().UnixNano())), // Consider a more meaningful key
		Value: payload,
	}

	return writer.WriteMessages(context.Background(), msg)
}
