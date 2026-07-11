package outbox

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer publishes outbox rows to Kafka. The topic is carried per message, so
// one Producer serves every topic this service produces.
type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Balancer:               &kafka.Hash{}, // key → partition: per-aggregate ordering
			RequiredAcks:           kafka.RequireAll,
			AllowAutoTopicCreation: true,
			BatchTimeout:           10 * time.Millisecond,
		},
	}
}

// Publish sends one message and returns once the brokers acknowledge it.
func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
