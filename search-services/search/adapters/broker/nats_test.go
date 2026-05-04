package broker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runMockNatsServer() *server.Server {
	opts := natstest.DefaultTestOptions
	return natstest.RunServer(&opts)
}

func TestSubscriber_Subscribe(t *testing.T) {
	ns := runMockNatsServer()
	defer ns.Shutdown()

	logger := slog.Default()
	url := ns.ClientURL()

	sub, err := NewSubscriber(url, logger)
	require.NoError(t, err)
	defer func() { _ = sub.Close() }()

	t.Run("MessageTriggerCallback", func(t *testing.T) {
		var callCount atomic.Int32
		subject := "test.subject"

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := sub.Subscribe(ctx, subject, func() {
			callCount.Add(1)
		})
		require.NoError(t, err)

		nc, err := nats.Connect(url)
		require.NoError(t, err)
		defer nc.Close()

		err = nc.Publish(subject, []byte("update"))
		require.NoError(t, err)

		assert.Eventually(t, func() bool {
			return callCount.Load() == 1
		}, 1*time.Second, 10*time.Millisecond, "callback should be called once")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		subject := "cancel.subject"
		ctx, cancel := context.WithCancel(context.Background())

		err := sub.Subscribe(ctx, subject, func() {})
		require.NoError(t, err)
		cancel()
		time.Sleep(50 * time.Millisecond)
	})
}

func TestNewSubscriber_Error(t *testing.T) {
	t.Run("InvalidURL", func(t *testing.T) {
		_, err := NewSubscriber("nats://localhost:9999", slog.Default())
		assert.Error(t, err, "expected error for invalid URL")
	})
}

func TestSubscriber_Subscribe_ChanSubscribeError(t *testing.T) {
	ns := runMockNatsServer()
	defer ns.Shutdown()

	logger := slog.Default()
	url := ns.ClientURL()

	sub, err := NewSubscriber(url, logger)
	require.NoError(t, err)
	_ = sub.Close()

	err = sub.Subscribe(context.Background(), "any.subject", func() {})
	assert.Error(t, err, "expected error when subscribing on closed connection")
}

func TestSubscriber_Subscribe_UnsubscribeError(t *testing.T) {
	ns := runMockNatsServer()
	defer ns.Shutdown()

	logger := slog.Default()
	url := ns.ClientURL()

	sub, err := NewSubscriber(url, logger)
	require.NoError(t, err)
	defer func() { _ = sub.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	err = sub.Subscribe(ctx, "test.unsub.error", func() {})
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	sub.nc.Close()
	cancel()
	time.Sleep(100 * time.Millisecond)
}
