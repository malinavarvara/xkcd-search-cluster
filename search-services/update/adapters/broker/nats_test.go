package broker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNatsConn struct {
	publishErr error
	flushErr   error
	lastErr    error
}

func (m *mockNatsConn) Publish(subj string, data []byte) error { return m.publishErr }
func (m *mockNatsConn) Flush() error                           { return m.flushErr }
func (m *mockNatsConn) LastError() error                       { return m.lastErr }
func (m *mockNatsConn) Close()                                 {}

func runTestNatsServer() (*server.Server, string) {
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		panic(err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		panic("nats-server didn't start")
	}
	return s, s.ClientURL()
}

func TestNatsPublisher_PublishUpdate(t *testing.T) {
	ns, url := runTestNatsServer()
	defer ns.Shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err, "failed to connect subscriber")
	defer nc.Close()

	msgs := make(chan *nats.Msg, 1)
	sub, err := nc.ChanSubscribe("xkcd.db.updated", msgs)
	require.NoError(t, err, "failed to subscribe")
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			t.Logf("failed to unsubscribe: %v", err)
		}
	}()

	publisher, err := NewNatsPublisher(url)
	require.NoError(t, err, "failed to create publisher")
	defer publisher.Close()

	t.Run("Success", func(t *testing.T) {
		err := publisher.PublishUpdate(context.Background())
		assert.NoError(t, err, "PublishUpdate failed")

		select {
		case msg := <-msgs:
			assert.Equal(t, "trigger_reindex", string(msg.Data), "unexpected payload")
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for nats message")
		}
	})

	t.Run("ConnectionError", func(t *testing.T) {
		publisher.Close()

		err := publisher.PublishUpdate(context.Background())
		assert.Error(t, err, "expected error due to closed connection")
	})
}

func TestNewNatsPublisher_Error(t *testing.T) {
	t.Run("InvalidURL", func(t *testing.T) {
		_, err := NewNatsPublisher("nats://127.0.0.1:1")
		assert.Error(t, err, "expected error for invalid nats url")
	})
}

func TestNatsPublisher_PublishUpdate_FlushError(t *testing.T) {
	mock := &mockNatsConn{flushErr: fmt.Errorf("flush failed")}
	pub := &NatsPublisher{nc: mock}
	err := pub.PublishUpdate(context.Background())
	assert.EqualError(t, err, "nats flush error: flush failed")
}

func TestNatsPublisher_PublishUpdate_LastError(t *testing.T) {
	mock := &mockNatsConn{lastErr: fmt.Errorf("async error")}
	pub := &NatsPublisher{nc: mock}
	err := pub.PublishUpdate(context.Background())
	assert.EqualError(t, err, "nats last error: async error")
}
