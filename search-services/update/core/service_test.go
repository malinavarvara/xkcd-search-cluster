package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDeps struct {
	lastIDFunc  func() (int, error)
	getFunc     func(int) (XKCDInfo, error)
	dbIDsFunc   func() ([]int, error)
	dbAddFunc   func(Comics) error
	normFunc    func(string) ([]string, error)
	publishFunc func() error
	dbStatsFunc func() (DBStats, error)
	dbDropFunc  func() error
}

func (m *mockDeps) LastID(context.Context) (int, error) {
	if m.lastIDFunc == nil {
		return 0, nil
	}
	return m.lastIDFunc()
}
func (m *mockDeps) Get(context.Context, int) (XKCDInfo, error) {
	if m.getFunc == nil {
		return XKCDInfo{}, nil
	}
	return m.getFunc(0)
}

func (m *mockDeps) IDs(ctx context.Context) ([]int, error) {
	if m.dbIDsFunc == nil {
		return []int{}, nil
	}
	return m.dbIDsFunc()
}
func (m *mockDeps) Add(ctx context.Context, c Comics) error {
	if m.dbAddFunc == nil {
		return nil
	}
	return m.dbAddFunc(c)
}
func (m *mockDeps) Norm(ctx context.Context, p string) ([]string, error) {
	if m.normFunc == nil {
		return []string{}, nil
	}
	return m.normFunc(p)
}
func (m *mockDeps) PublishUpdate(ctx context.Context) error {
	if m.publishFunc == nil {
		return nil
	}
	return m.publishFunc()
}
func (m *mockDeps) Stats(ctx context.Context) (DBStats, error) {
	if m.dbStatsFunc == nil {
		return DBStats{}, nil
	}
	return m.dbStatsFunc()
}
func (m *mockDeps) Drop(ctx context.Context) error {
	if m.dbDropFunc == nil {
		return nil
	}
	return m.dbDropFunc()
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestUpdate_FullCoverage(t *testing.T) {
	logger := testLogger()

	t.Run("Success_NewComics", func(t *testing.T) {
		m := &mockDeps{
			lastIDFunc: func() (int, error) { return 2, nil },
			dbIDsFunc:  func() ([]int, error) { return []int{1}, nil },
			getFunc: func(id int) (XKCDInfo, error) {
				return XKCDInfo{ID: id, Year: "2024", Month: "04", Day: "27"}, nil
			},
			normFunc:    func(string) ([]string, error) { return []string{"test"}, nil },
			dbAddFunc:   func(Comics) error { return nil },
			publishFunc: func() error { return nil },
		}
		svc, err := NewService(logger, m, m, m, 2, m)
		require.NoError(t, err)
		err = svc.Update(context.Background())
		assert.NoError(t, err)
	})

	t.Run("No_New_Comics", func(t *testing.T) {
		m := &mockDeps{
			lastIDFunc: func() (int, error) { return 1, nil },
			dbIDsFunc:  func() ([]int, error) { return []int{1}, nil },
		}
		svc, err := NewService(logger, m, m, m, 1, m)
		require.NoError(t, err)
		err = svc.Update(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Error_Paths_To_Green_Returns", func(t *testing.T) {
		m := &mockDeps{lastIDFunc: func() (int, error) { return 0, fmt.Errorf("api fail") }}
		svc, _ := NewService(logger, m, m, m, 1, m)
		_ = svc.Update(context.Background())

		m.lastIDFunc = func() (int, error) { return 1, nil }
		m.dbIDsFunc = func() ([]int, error) { return nil, fmt.Errorf("db fail") }
		_ = svc.Update(context.Background())

		m.dbIDsFunc = func() ([]int, error) { return []int{}, nil }
		m.getFunc = func(id int) (XKCDInfo, error) {
			return XKCDInfo{ID: id, Year: "bad"}, nil
		}
		m.normFunc = func(string) ([]string, error) { return []string{}, nil }
		m.dbAddFunc = func(Comics) error { return nil }
		_ = svc.Update(context.Background())

		m.getFunc = func(id int) (XKCDInfo, error) {
			return XKCDInfo{ID: id, Year: "2024", Month: "1", Day: "1"}, nil
		}
		m.publishFunc = func() error { return fmt.Errorf("pub fail") }
		_ = svc.Update(context.Background())
	})

	t.Run("Stats_And_Status", func(t *testing.T) {
		m := &mockDeps{
			dbStatsFunc: func() (DBStats, error) { return DBStats{}, nil },
			lastIDFunc:  func() (int, error) { return 100, nil },
		}
		svc, _ := NewService(logger, m, m, m, 1, m)

		_ = svc.Status(context.Background())
		_, _ = svc.Stats(context.Background())

		m.dbStatsFunc = func() (DBStats, error) { return DBStats{}, fmt.Errorf("fail") }
		_, _ = svc.Stats(context.Background())
	})

	t.Run("Drop_Coverage", func(t *testing.T) {
		m := &mockDeps{
			dbDropFunc:  func() error { return nil },
			publishFunc: func() error { return nil },
		}
		svc, _ := NewService(logger, m, m, m, 1, m)
		_ = svc.Drop(context.Background())

		m.dbDropFunc = func() error { return fmt.Errorf("fail") }
		_ = svc.Drop(context.Background())
	})
}

func TestNewService_InvalidConcurrency(t *testing.T) {
	logger := testLogger()
	var m mockDeps
	_, err := NewService(logger, &m, &m, &m, 0, &m)
	assert.Error(t, err)
	_, err = NewService(logger, &m, &m, &m, -1, &m)
	assert.Error(t, err)
}

func TestUpdate_AlreadyRunning(t *testing.T) {
	logger := testLogger()
	start := make(chan struct{})
	m := &mockDeps{
		lastIDFunc: func() (int, error) { return 10, nil },
		dbIDsFunc:  func() ([]int, error) { return []int{}, nil },
		getFunc: func(int) (XKCDInfo, error) {
			<-start
			return XKCDInfo{}, nil
		},
		normFunc:  func(string) ([]string, error) { return []string{}, nil },
		dbAddFunc: func(Comics) error { return nil },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- svc.Update(context.Background()) }()

	time.Sleep(50 * time.Millisecond)

	err = svc.Update(context.Background())
	assert.Equal(t, ErrUpdateAlreadyRunning, err)

	close(start)
	<-errCh
}

func TestUpdate_ContextCancelDuringProcessing(t *testing.T) {
	logger := testLogger()
	m := &mockDeps{
		lastIDFunc: func() (int, error) { return 1, nil },
		dbIDsFunc:  func() ([]int, error) { return []int{}, nil },
		getFunc: func(int) (XKCDInfo, error) {
			time.Sleep(1 * time.Second)
			return XKCDInfo{}, nil
		},
		normFunc:  func(string) ([]string, error) { return []string{}, nil },
		dbAddFunc: func(Comics) error { return nil },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = svc.Update(ctx)
	assert.Equal(t, context.Canceled, err)
}

func TestUpdate_ProcessComicError(t *testing.T) {
	logger := testLogger()
	m := &mockDeps{
		lastIDFunc: func() (int, error) { return 1, nil },
		dbIDsFunc:  func() ([]int, error) { return []int{}, nil },
		getFunc:    func(int) (XKCDInfo, error) { return XKCDInfo{}, fmt.Errorf("network error") },
		normFunc:   func(string) ([]string, error) { return []string{}, nil },
		dbAddFunc:  func(Comics) error { return nil },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)
	err = svc.Update(context.Background())
	assert.Error(t, err)
}

func TestStats_LastIDError(t *testing.T) {
	logger := testLogger()
	m := &mockDeps{
		dbStatsFunc: func() (DBStats, error) { return DBStats{WordsTotal: 100}, nil },
		lastIDFunc:  func() (int, error) { return 0, fmt.Errorf("api down") },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	stats, err := svc.Stats(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, stats.ComicsTotal)
}

func TestStatus_Running(t *testing.T) {
	logger := testLogger()
	start := make(chan struct{})
	m := &mockDeps{
		lastIDFunc: func() (int, error) { return 1, nil },
		dbIDsFunc:  func() ([]int, error) { return []int{}, nil },
		getFunc: func(int) (XKCDInfo, error) {
			<-start
			return XKCDInfo{}, nil
		},
		normFunc:  func(string) ([]string, error) { return []string{}, nil },
		dbAddFunc: func(Comics) error { return nil },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	go func() { _ = svc.Update(context.Background()) }()

	timeout := time.After(1 * time.Second)
	for !svc.isRunning.Load() {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for isRunning")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	assert.Equal(t, StatusRunning, svc.Status(context.Background()))
	close(start)
}

func TestDrop_PublishError(t *testing.T) {
	logger := testLogger()
	m := &mockDeps{
		dbDropFunc:  func() error { return nil },
		publishFunc: func() error { return fmt.Errorf("nats error") },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	err = svc.Drop(context.Background())
	assert.NoError(t, err)
}

func TestProcessComic_NotFound(t *testing.T) {
	logger := testLogger()
	var addedStub bool
	m := &mockDeps{
		getFunc: func(int) (XKCDInfo, error) { return XKCDInfo{}, ErrNotFound },
		dbAddFunc: func(c Comics) error {
			if c.Num == 42 && len(c.Words) == 0 {
				addedStub = true
			}
			return nil
		},
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	err = svc.processComic(context.Background(), 42)
	assert.NoError(t, err)
	assert.True(t, addedStub, "stub comic should be added")
}

func TestParseDate_Errors(t *testing.T) {
	_, err := parseDate("invalid", "1", "1")
	assert.Error(t, err)
	_, err = parseDate("2024", "invalid", "1")
	assert.Error(t, err)
	_, err = parseDate("2024", "1", "invalid")
	assert.Error(t, err)
}

func TestProcessComic_PhraseTooLong(t *testing.T) {
	logger := testLogger()
	long := strings.Repeat("A", 5000)
	var truncatedLength int
	m := &mockDeps{
		getFunc: func(int) (XKCDInfo, error) {
			return XKCDInfo{
				ID:          1,
				Title:       long,
				SafeTitle:   long,
				Description: long,
				Alt:         long,
				Year:        "2024", Month: "1", Day: "1",
			}, nil
		},
		normFunc: func(p string) ([]string, error) {
			truncatedLength = len(p)
			return []string{}, nil
		},
		dbAddFunc: func(Comics) error { return nil },
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	err = svc.processComic(context.Background(), 1)
	assert.NoError(t, err)
	assert.True(t, truncatedLength <= 16384, "phrase should be truncated to <=16384, got %d", truncatedLength)
}

func TestProcessComic_NormError(t *testing.T) {
	logger := testLogger()
	m := &mockDeps{
		getFunc: func(int) (XKCDInfo, error) {
			return XKCDInfo{ID: 1, Year: "2024", Month: "1", Day: "1"}, nil
		},
		normFunc: func(string) ([]string, error) {
			return nil, fmt.Errorf("normalization service unavailable")
		},
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	err = svc.processComic(context.Background(), 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "normalize words")
}

func TestProcessComic_DBAddError(t *testing.T) {
	logger := testLogger()
	m := &mockDeps{
		getFunc: func(int) (XKCDInfo, error) {
			return XKCDInfo{ID: 1, Year: "2024", Month: "1", Day: "1"}, nil
		},
		normFunc: func(string) ([]string, error) { return []string{"hello"}, nil },
		dbAddFunc: func(Comics) error {
			return fmt.Errorf("database insert failed")
		},
	}
	svc, err := NewService(logger, m, m, m, 1, m)
	require.NoError(t, err)

	err = svc.processComic(context.Background(), 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save comic")
}
