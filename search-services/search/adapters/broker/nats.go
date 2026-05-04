package broker

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type Subscriber struct {
	nc  *nats.Conn
	log *slog.Logger
}

func NewSubscriber(url string, log *slog.Logger) (*Subscriber, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &Subscriber{nc: nc, log: log}, nil
}

func (s *Subscriber) Subscribe(ctx context.Context, subject string, callback func()) error {
	msgs := make(chan *nats.Msg, 1)

	sub, err := s.nc.ChanSubscribe(subject, msgs)
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			if err := sub.Unsubscribe(); err != nil {
				s.log.Error("failed to unsubscribe from NATS", "error", err)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-msgs:
				s.log.Info("received update trigger from NATS")
				callback()
			}
		}
	}()

	return nil
}

func (s *Subscriber) Close() error {
	return s.nc.Drain()
}
