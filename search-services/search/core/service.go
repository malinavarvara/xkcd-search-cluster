package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync/atomic"
	"time"
)

type service struct {
	words       Words
	repo        ComicsRepository
	logger      *slog.Logger
	index       atomic.Value
	lastUpdated atomic.Int64
	indexTTL    time.Duration
	natsTrigger chan struct{}
}

func NewService(words Words, repo ComicsRepository, logger *slog.Logger, indexTTL time.Duration) Searcher {
	s := &service{
		words:       words,
		repo:        repo,
		logger:      logger,
		indexTTL:    indexTTL,
		natsTrigger: make(chan struct{}, 1),
	}
	s.index.Store(make(map[string]map[int]int))
	return s
}

func (s *service) TriggerRebuild() {
	select {
	case s.natsTrigger <- struct{}{}:
	default:

	}
}

func (s *service) RunBackgroundIndexer(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := s.BuildIndex(ctx); err != nil {
		s.logger.Error("initial index build failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logger.Info("scheduled index rebuild (self-healing)")
			if err := s.BuildIndex(ctx); err != nil {
				s.logger.Error("scheduled rebuild failed", "error", err)
			}
		case <-s.natsTrigger:
			s.logger.Info("index rebuild triggered by NATS")
			if err := s.BuildIndex(ctx); err == nil {
				ticker.Reset(interval)
			}
		}
	}
}

func (s *service) BuildIndex(ctx context.Context) error {
	s.logger.Info("rebuilding index via atomic store")

	rawData, err := s.repo.GetAllComicsWords(ctx)
	if err != nil {
		return err
	}

	newIndex := make(map[string]map[int]int, len(rawData))
	for word, ids := range rawData {
		if _, ok := newIndex[word]; !ok {
			newIndex[word] = make(map[int]int)
		}
		for _, id := range ids {
			newIndex[word][id]++
		}
	}

	s.index.Store(newIndex)
	s.lastUpdated.Store(time.Now().Unix())
	s.logger.Info("index updated successfully")
	return nil
}

func (s *service) ISearch(ctx context.Context, phrase string, limit int) ([]Comics, int, error) {
	lastUpd := s.lastUpdated.Load()
	if lastUpd > 0 && time.Since(time.Unix(lastUpd, 0)) > s.indexTTL {
		s.logger.Warn("search index is stale", "age", time.Since(time.Unix(lastUpd, 0)))
	}
	if phrase == "" {
		return nil, 0, ErrEmptyPhrase
	}
	start := time.Now()
	defer func() {
		s.logger.Info("ISearch latency",
			"phrase", phrase,
			"limit", limit,
			"ms", time.Since(start).Milliseconds(),
		)
	}()
	rawIdx := s.index.Load()
	idx, ok := rawIdx.(map[string]map[int]int)
	if !ok {
		return nil, 0, errors.New("failed to load index: wrong format")
	}

	words, err := s.words.Norm(ctx, phrase)
	if err != nil || len(words) == 0 {
		return []Comics{}, 0, nil
	}

	type resultScore struct {
		matchedUniqueWords int
		totalOccurrences   int
	}

	relevance := make(map[int]resultScore, 100)

	for _, word := range words {
		if counts, found := idx[word]; found {
			for id, count := range counts {
				score := relevance[id]
				score.matchedUniqueWords++
				score.totalOccurrences += count
				relevance[id] = score
			}
		}
	}

	if len(relevance) == 0 {
		return []Comics{}, 0, nil
	}

	type entry struct {
		id    int
		score resultScore
	}
	sorted := make([]entry, 0, len(relevance))
	for id, score := range relevance {
		sorted = append(sorted, entry{id, score})
	}

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].score.matchedUniqueWords != sorted[j].score.matchedUniqueWords {
			return sorted[i].score.matchedUniqueWords > sorted[j].score.matchedUniqueWords
		}
		if sorted[i].score.totalOccurrences != sorted[j].score.totalOccurrences {
			return sorted[i].score.totalOccurrences > sorted[j].score.totalOccurrences
		}
		return sorted[i].id > sorted[j].id
	})

	total := len(sorted)
	if limit > total {
		limit = total
	}
	if limit <= 0 {
		return []Comics{}, total, nil
	}

	targetIDs := make([]int, 0, limit)
	for i := 0; i < limit; i++ {
		targetIDs = append(targetIDs, sorted[i].id)
	}

	comics, err := s.repo.GetComicsByIDs(ctx, targetIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch comics details: %w", err)
	}

	idToComic := make(map[int]Comics)
	for _, c := range comics {
		idToComic[c.Num] = c
	}

	finalResult := make([]Comics, 0, len(targetIDs))
	for _, id := range targetIDs {
		if c, ok := idToComic[id]; ok {
			finalResult = append(finalResult, c)
		}
	}

	return finalResult, total, nil
}

func (s *service) Search(ctx context.Context, phrase string, limit int) ([]Comics, int, error) {
	start := time.Now()
	defer func() {
		s.logger.Info("Search latency",
			"phrase", phrase,
			"limit", limit,
			"ms", time.Since(start).Milliseconds(),
		)
	}()
	if phrase == "" {
		return nil, 0, ErrEmptyPhrase
	}

	normalizedWords, err := s.words.Norm(ctx, phrase)
	if err != nil {
		return nil, 0, fmt.Errorf("normalize phrase: %w", err)
	}

	if len(normalizedWords) == 0 {
		return []Comics{}, 0, nil
	}

	comics, err := s.repo.SearchComics(ctx, normalizedWords, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("search comics: %w", err)
	}

	total, err := s.repo.CountSearchResults(ctx, normalizedWords)
	if err != nil {
		return nil, 0, fmt.Errorf("count results: %w", err)
	}

	return comics, total, nil
}
