package kafka

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log/slog"
	"time"

	"backend/messages-service/internal/config"
)

type Producer struct {
	writer *kafka.Writer
	log    *slog.Logger
	cfg    config.KafkaConfig
}

func NewProducer(cfg config.KafkaConfig, log *slog.Logger) (*Producer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("no kafka brokers provided")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: parseAcks(cfg.Producer.Acks),
		Async:        false,
	}

	p := &Producer{
		writer: writer,
		log:    log,
		cfg:    cfg,
	}

	log.Info("kafka producer initialized",
		slog.Any("brokers", cfg.Brokers),
		slog.String("acks", cfg.Producer.Acks),
	)

	return p, nil
}

func (p *Producer) Send(ctx context.Context, topic string, key string, value []byte) error {
	if topic == "" {
		return fmt.Errorf("topic is empty")
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
		Time:  time.Now().UTC(),
	}

	timeout := p.cfg.Producer.Timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.log.Error("failed to send message to kafka",
			slog.Any("error", err),
			slog.String("topic", topic),
			slog.String("key", key),
		)
		return err
	}

	p.log.Debug("message sent to kafka",
		slog.String("topic", topic),
		slog.String("key", key),
	)

	return nil
}

func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		p.log.Warn("failed to close kafka writer", slog.Any("error", err))
		return err
	}
	return nil
}

func parseAcks(acks string) kafka.RequiredAcks {
	switch acks {
	case "none":
		return kafka.RequireNone
	case "leader":
		return kafka.RequireOne
	case "all":
		fallthrough
	default:
		return kafka.RequireAll
	}
}
