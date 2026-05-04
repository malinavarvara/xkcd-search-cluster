package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubWords struct{}

func (s *stubWords) Norm(ctx context.Context, phrase string) ([]string, error) {
	if phrase == "" {
		return nil, nil
	}
	return []string{"linux"}, nil
}

type stubRepo struct{}

func (s *stubRepo) GetAllComicsWords(ctx context.Context) (map[string][]int, error) {
	return map[string][]int{"linux": {1, 1, 2}}, nil
}
func (s *stubRepo) GetComicsByIDs(ctx context.Context, ids []int) ([]Comics, error) {
	return []Comics{{Num: 1, ImgURL: "url1"}, {Num: 2, ImgURL: "url2"}}, nil
}
func (s *stubRepo) SearchComics(ctx context.Context, words []string, limit int) ([]Comics, error) {
	return []Comics{{Num: 1}}, nil
}
func (s *stubRepo) CountSearchResults(ctx context.Context, words []string) (int, error) {
	return 1, nil
}

func TestService_Final(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(&stubWords{}, &stubRepo{}, logger, time.Hour)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err, "BuildIndex failed")

	t.Run("ISearch_Ranking", func(t *testing.T) {
		res, total, err := svc.ISearch(context.Background(), "linux", 10)
		require.NoError(t, err, "ISearch error")
		assert.Equal(t, 2, total, "Expected 2 results")
		if len(res) > 0 {
			assert.Equal(t, 1, res[0].Num, "Ranking fail: comic 1 should be first")
		}
	})

	t.Run("Plain_Search", func(t *testing.T) {
		_, _, err := svc.Search(context.Background(), "linux", 10)
		assert.NoError(t, err, "Search should not fail")
	})

	t.Run("Background", func(t *testing.T) {
		svc.TriggerRebuild()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		svc.RunBackgroundIndexer(ctx, 5*time.Millisecond)
	})
}

type errorWords struct {
	err   error
	words []string
}

func (e *errorWords) Norm(ctx context.Context, phrase string) ([]string, error) {
	return e.words, e.err
}

type errorRepo struct {
	getAllErr      error
	getByIDsErr    error
	searchErr      error
	countErr       error
	getAllResult   map[string][]int
	getByIDsResult []Comics
	searchResult   []Comics
	countResult    int
}

func (e *errorRepo) GetAllComicsWords(ctx context.Context) (map[string][]int, error) {
	return e.getAllResult, e.getAllErr
}
func (e *errorRepo) GetComicsByIDs(ctx context.Context, ids []int) ([]Comics, error) {
	return e.getByIDsResult, e.getByIDsErr
}
func (e *errorRepo) SearchComics(ctx context.Context, words []string, limit int) ([]Comics, error) {
	return e.searchResult, e.searchErr
}
func (e *errorRepo) CountSearchResults(ctx context.Context, words []string) (int, error) {
	return e.countResult, e.countErr
}

func TestRunBackgroundIndexer_InitialError(t *testing.T) {
	repo := &errorRepo{getAllErr: fmt.Errorf("db down")}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunBackgroundIndexer(ctx, 10*time.Millisecond)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
}

func TestRunBackgroundIndexer_ScheduledError(t *testing.T) {
	repo := &errorRepo{getAllErr: fmt.Errorf("db down")}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunBackgroundIndexer(ctx, 5*time.Millisecond)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
}

func TestBuildIndex_Error(t *testing.T) {
	repo := &errorRepo{getAllErr: fmt.Errorf("get all failed")}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour).(*service)

	err := svc.BuildIndex(context.Background())
	assert.Error(t, err, "expected error")
}

func TestISearch_StaleIndex(t *testing.T) {
	words := &stubWords{}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Millisecond).(*service)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)

	_, _, err = svc.ISearch(context.Background(), "linux", 10)
	assert.NoError(t, err, "stale index should not cause error")
}

func TestISearch_EmptyPhrase(t *testing.T) {
	words := &stubWords{}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	_, _, err := svc.ISearch(context.Background(), "", 10)
	assert.Equal(t, ErrEmptyPhrase, err, "expected ErrEmptyPhrase")
}

func TestISearch_InvalidIndexType(t *testing.T) {
	words := &stubWords{}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := &service{
		words:       words,
		repo:        repo,
		logger:      logger,
		indexTTL:    time.Hour,
		natsTrigger: make(chan struct{}, 1),
	}
	svc.index.Store("invalid")

	_, _, err := svc.ISearch(context.Background(), "linux", 10)
	assert.EqualError(t, err, "failed to load index: wrong format")
}

func TestISearch_NormalizerErrorOrEmpty(t *testing.T) {
	wordsErr := &errorWords{err: fmt.Errorf("norm error")}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(wordsErr, repo, logger, time.Hour)

	comics, total, err := svc.ISearch(context.Background(), "test", 10)
	assert.NoError(t, err, "error should be swallowed")
	assert.Empty(t, comics)
	assert.Zero(t, total)

	wordsEmpty := &errorWords{words: []string{}}
	svc2 := NewService(wordsEmpty, repo, logger, time.Hour)
	comics2, total2, err2 := svc2.ISearch(context.Background(), "test", 10)
	assert.NoError(t, err2)
	assert.Empty(t, comics2)
	assert.Zero(t, total2)
}

func TestISearch_NoRelevance(t *testing.T) {
	words := &stubWords{}
	repo := &errorRepo{getAllResult: map[string][]int{"other": {1}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	comics, total, err := svc.ISearch(context.Background(), "linux", 10)
	assert.NoError(t, err)
	assert.Empty(t, comics)
	assert.Zero(t, total)
}

func TestISearch_SortingByTotalOccurrencesAndID(t *testing.T) {
	rawData := map[string][]int{"linux": {1, 1, 2, 2, 2, 3}}
	repo := &errorRepo{
		getAllResult:   rawData,
		getByIDsResult: []Comics{{Num: 1, ImgURL: "url1"}, {Num: 2, ImgURL: "url2"}, {Num: 3, ImgURL: "url3"}},
	}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour).(*service)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err)

	comics, total, err := svc.ISearch(context.Background(), "linux", 3)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, comics, 3)

	expectedOrder := []int{2, 1, 3}
	for i, exp := range expectedOrder {
		assert.Equal(t, exp, comics[i].Num, "position %d", i)
	}
}

func TestSearch_CountError(t *testing.T) {
	repo := &errorRepo{
		searchErr:    nil,
		countErr:     fmt.Errorf("count failed"),
		searchResult: []Comics{{Num: 1}},
	}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	_, _, err := svc.Search(context.Background(), "linux", 10)
	assert.Error(t, err, "expected count error")
}

func TestSearch_EmptyPhrase(t *testing.T) {
	words := &stubWords{}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	_, _, err := svc.Search(context.Background(), "", 10)
	assert.Equal(t, ErrEmptyPhrase, err)
}

func TestSearch_NormalizeError(t *testing.T) {
	words := &errorWords{err: fmt.Errorf("norm failed")}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	_, _, err := svc.Search(context.Background(), "test", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "normalize phrase")
}

func TestSearch_EmptyNormalizedWords(t *testing.T) {
	words := &errorWords{words: []string{}}
	repo := &stubRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	comics, total, err := svc.Search(context.Background(), "test", 10)
	assert.NoError(t, err)
	assert.Empty(t, comics)
	assert.Zero(t, total)
}

func TestSearch_SearchComicsError(t *testing.T) {
	repo := &errorRepo{searchErr: fmt.Errorf("db search error")}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	_, _, err := svc.Search(context.Background(), "linux", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search comics")
}

func TestISearch_GetComicsByIDsError(t *testing.T) {
	repo := &errorRepo{
		getAllResult: map[string][]int{"linux": {1, 2}},
		getByIDsErr:  fmt.Errorf("failed to fetch by ids"),
	}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err)

	_, _, err = svc.ISearch(context.Background(), "linux", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch comics details")
}

func TestISearch_ZeroLimit(t *testing.T) {
	repo := &stubRepo{}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err)

	comics, total, err := svc.ISearch(context.Background(), "linux", 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Empty(t, comics)
}

func TestISearch_SortingByIDWhenMatchedUniqueWordsEqual(t *testing.T) {
	rawData := map[string][]int{"linux": {5, 3}}
	repo := &errorRepo{
		getAllResult:   rawData,
		getByIDsResult: []Comics{{Num: 3, ImgURL: "url3"}, {Num: 5, ImgURL: "url5"}},
	}
	words := &stubWords{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(words, repo, logger, time.Hour)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err)

	comics, total, err := svc.ISearch(context.Background(), "linux", 2)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, []int{5, 3}, []int{comics[0].Num, comics[1].Num})
}

type dynamicWords struct {
	normFunc func(ctx context.Context, phrase string) ([]string, error)
}

func (d *dynamicWords) Norm(ctx context.Context, phrase string) ([]string, error) {
	return d.normFunc(ctx, phrase)
}

func TestISearch_SortingByMatchedUniqueWords(t *testing.T) {
	rawData := map[string][]int{
		"linux":  {1, 2},
		"kernel": {1},
		"open":   {3},
	}
	repo := &errorRepo{
		getAllResult: rawData,
		getByIDsResult: []Comics{
			{Num: 1, ImgURL: "url1"},
			{Num: 2, ImgURL: "url2"},
			{Num: 3, ImgURL: "url3"},
		},
	}

	wordsMock := &dynamicWords{
		normFunc: func(ctx context.Context, phrase string) ([]string, error) {
			if phrase == "linux kernel open" {
				return []string{"linux", "kernel", "open"}, nil
			}
			return []string{}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(wordsMock, repo, logger, time.Hour).(*service)

	err := svc.BuildIndex(context.Background())
	require.NoError(t, err)

	comics, total, err := svc.ISearch(context.Background(), "linux kernel open", 3)
	require.NoError(t, err)
	assert.Equal(t, 3, total)

	expectedIDs := []int{1, 3, 2}
	for i, exp := range expectedIDs {
		assert.Equal(t, exp, comics[i].Num, "position %d", i)
	}
}
