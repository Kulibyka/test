package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

// EnsureTopics creates the required topics if they do not already exist.
func EnsureTopics(ctx context.Context, brokers []string, log *slog.Logger, topics ...string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("no kafka brokers provided")
	}

	if len(topics) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("dial kafka broker: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get kafka controller: %w", err)
	}

	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	controllerConn, err := kafka.DialContext(ctx, "tcp", controllerAddr)
	if err != nil {
		return fmt.Errorf("dial kafka controller: %w", err)
	}
	defer controllerConn.Close()

	topicConfigs := make([]kafka.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		topicConfigs = append(topicConfigs, kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}

	if err := controllerConn.CreateTopics(topicConfigs...); err != nil {
		return fmt.Errorf("create topics: %w", err)
	}

	log.Info("kafka topics ensured", slog.Any("topics", topics))
	return nil
}
