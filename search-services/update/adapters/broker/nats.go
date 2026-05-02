package broker

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
)

type natsConn interface {
	Publish(subj string, data []byte) error
	Flush() error
	LastError() error
	Close()
}

type NatsPublisher struct {
	nc natsConn
}

func NewNatsPublisher(url string) (*NatsPublisher, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return &NatsPublisher{nc: nc}, nil
}

func (p *NatsPublisher) PublishUpdate(ctx context.Context) error {
	err := p.nc.Publish("xkcd.db.updated", []byte("trigger_reindex"))
	if err != nil {
		return fmt.Errorf("nats publish error: %w", err)
	}

	if err := p.nc.Flush(); err != nil {
		return fmt.Errorf("nats flush error: %w", err)
	}
	if err := p.nc.LastError(); err != nil {
		return fmt.Errorf("nats last error: %w", err)
	}
	return nil
}

func (p *NatsPublisher) Close() {
	p.nc.Close()
}
