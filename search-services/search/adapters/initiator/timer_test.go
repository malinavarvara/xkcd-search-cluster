package initiator

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"yadro.com/course/search/core"
)

type MockSearcher struct {
	core.Searcher
	buildCalls atomic.Int32
}

func (m *MockSearcher) BuildIndex(ctx context.Context) error {
	m.buildCalls.Add(1)
	return nil
}
func TestTimer_Start(t *testing.T) {
	t.Run("InitialAndPeriodicBuild", func(t *testing.T) {
		mock := &MockSearcher{}
		ttl := 50 * time.Millisecond
		logger := slog.New(slog.NewTextHandler(openDevNull(), nil))

		timer := NewTimer(mock, ttl, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()

		go timer.Start(ctx)

		<-ctx.Done()

		calls := mock.buildCalls.Load()
		assert.GreaterOrEqual(t, calls, int32(2), "Должно быть как минимум 2 вызова (начальный + периодический)")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		mock := &MockSearcher{}
		timer := NewTimer(mock, 1*time.Hour, slog.Default())

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			timer.Start(ctx)
			close(done)
		}()

		cancel()

		select {
		case <-done:
			assert.Equal(t, int32(1), mock.buildCalls.Load(), "Должен быть только начальный вызов")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Таймер не завершился после отмены контекста")
		}
	})
}

func openDevNull() *os.File {
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return os.Stdout
	}
	return f
}
