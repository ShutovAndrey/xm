package broker

import (
	"context"
	"errors"
	"fmt"
	"time"

	gokafka "gitlab.com/shutovAndrey/go-kafka-client"
)

// Producer publishes to a single go-kafka topic. It connects lazily, bounds
// every call by timeout and reconnects after an error. Not safe for concurrent
// use: the outbox relay is its only caller.
type Producer struct {
	host, port, topic string
	timeout           time.Duration
	w                 *gokafka.Writer
}

func NewProducer(host, port, topic string, timeout time.Duration) *Producer {
	return &Producer{host: host, port: port, topic: topic, timeout: timeout}
}

func (p *Producer) Publish(ctx context.Context, key, value []byte, headers map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if p.w == nil {
		w, err := gokafka.NewWriter(ctx, p.host, p.port, p.topic)
		if err != nil {
			return fmt.Errorf("connect to broker: %w", err)
		}
		p.w = w
	}

	hs := make([]gokafka.Header, 0, len(headers))
	for k, v := range headers {
		hs = append(hs, gokafka.Header{Key: k, Value: []byte(v)})
	}

	if err := p.w.WriteMessage(ctx, key, value, hs...); err != nil {
		return errors.Join(fmt.Errorf("publish: %w", err), p.Close())
	}
	return nil
}

func (p *Producer) Close() error {
	if p.w == nil {
		return nil
	}
	err := p.w.Close()
	p.w = nil
	return err
}
