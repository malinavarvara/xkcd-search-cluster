package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type Service struct {
	log         *slog.Logger
	db          DB
	xkcd        XKCD
	words       Words
	concurrency int
	isRunning   atomic.Bool
	publisher   EventPublisher
	appCtx      context.Context
}

func NewService(
	appCtx context.Context, log *slog.Logger, db DB, xkcd XKCD, words Words, concurrency int, publisher EventPublisher,
) (*Service, error) {
	if concurrency < 1 {
		return nil, fmt.Errorf("wrong concurrency specified: %d", concurrency)
	}
	return &Service{
		log:         log,
		db:          db,
		xkcd:        xkcd,
		words:       words,
		concurrency: concurrency,
		publisher:   publisher,
		appCtx:      appCtx,
	}, nil
}

func (s *Service) Update(_ context.Context) error {
	if s.isRunning.Swap(true) {
		return ErrUpdateAlreadyRunning
	}

	go func() {
		defer s.isRunning.Store(false)

		bgCtx, cancel := context.WithTimeout(s.appCtx, 30*time.Minute)
		defer cancel()

		s.log.Info("background update process started")
		if err := s.doUpdate(bgCtx); err != nil {
			s.log.Error("background update failed", "error", err)
			return
		}
		s.log.Info("background update finished successfully")
	}()

	return nil
}

func (s *Service) doUpdate(ctx context.Context) error {
	total, err := s.xkcd.LastID(ctx)
	if err != nil {
		return fmt.Errorf("get total comics: %w", err)
	}

	existingNums, err := s.db.IDs(ctx)
	if err != nil {
		return fmt.Errorf("get existing ids: %w", err)
	}

	existingMap := make(map[int]struct{}, len(existingNums))
	for _, num := range existingNums {
		existingMap[num] = struct{}{}
	}

	var numsToFetch []int
	for num := 1; num <= total; num++ {
		if _, exists := existingMap[num]; !exists {
			numsToFetch = append(numsToFetch, num)
		}
	}

	if len(numsToFetch) == 0 {
		s.log.Info("no new comics to fetch")
		return nil
	}

	s.log.Info("fetching new comics", "count", len(numsToFetch))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for _, num := range numsToFetch {
		num := num
		g.Go(func() error {
			if err := gCtx.Err(); err != nil {
				return err
			}

			if err := s.processComic(gCtx, num); err != nil {
				s.log.Error("failed to process comic", "num", num, "error", err)
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			s.log.Warn("update interrupted by application shutdown")
			return err
		}
		return fmt.Errorf("errgroup wait: %w", err)
	}

	if err := s.publisher.PublishUpdate(ctx); err != nil {
		s.log.Error("failed to publish update event", "error", err)
	}

	return nil
}

func (s *Service) Stats(ctx context.Context) (ServiceStats, error) {
	dbStats, err := s.db.Stats(ctx)
	if err != nil {
		return ServiceStats{}, err
	}

	total, err := s.xkcd.LastID(ctx)
	if err != nil {
		s.log.Warn("failed to get total comics count from xkcd", "error", err)
	}

	return ServiceStats{
		DBStats:     dbStats,
		ComicsTotal: total,
	}, nil
}

func (s *Service) Status(ctx context.Context) ServiceStatus {
	if s.isRunning.Load() {
		return StatusRunning
	}
	return StatusIdle
}

func (s *Service) Drop(ctx context.Context) error {
	if err := s.db.Drop(ctx); err != nil {
		return err
	}

	if err := s.publisher.PublishUpdate(ctx); err != nil {
		s.log.Error("failed to publish update event after drop", "error", err)
	}

	return nil
}

func (s *Service) processComic(ctx context.Context, num int) error {
	xkcdInfo, err := s.xkcd.Get(ctx, num)
	if err != nil {
		if errors.Is(err, ErrNotFound) || strings.Contains(err.Error(), "404") {
			s.log.Debug("comic skipped (404), saving stub for stats", "num", num)

			stub := Comics{
				Num:         num,
				PublishedAt: time.Now().UTC(),
				Words:       []string{},
			}
			return s.db.Add(ctx, stub)
		}
		return fmt.Errorf("get comic %d: %w", num, err)
	}

	publishedAt, err := parseDate(xkcdInfo.Year, xkcdInfo.Month, xkcdInfo.Day)
	if err != nil {
		s.log.Warn("failed to parse date, using today",
			"num", num, "year", xkcdInfo.Year, "month", xkcdInfo.Month, "day", xkcdInfo.Day, "error", err)
		publishedAt = time.Now().UTC().Truncate(24 * time.Hour)
	}

	var sb strings.Builder
	sb.WriteString(xkcdInfo.Title)
	sb.WriteByte(' ')
	sb.WriteString(xkcdInfo.SafeTitle)
	sb.WriteByte(' ')
	sb.WriteString(xkcdInfo.Description)
	sb.WriteByte(' ')
	sb.WriteString(xkcdInfo.Alt)

	phrase := sb.String()

	const maxInputLen = 16 * 1024
	if len(phrase) > maxInputLen {
		s.log.Warn("phrase too long, truncating", "num", num, "original_len", len(phrase))
		phrase = phrase[:maxInputLen]
	}

	normalizedWords, err := s.words.Norm(ctx, phrase)
	if err != nil {
		return fmt.Errorf("normalize words for %d: %w", num, err)
	}

	comic := Comics{
		Num:         xkcdInfo.ID,
		ImgURL:      xkcdInfo.URL,
		PublishedAt: publishedAt,
		Words:       normalizedWords,
	}

	if err := s.db.Add(ctx, comic); err != nil {
		return fmt.Errorf("save comic %d: %w", num, err)
	}

	return nil
}

func parseDate(year, month, day string) (time.Time, error) {
	y, err := strconv.Atoi(year)
	if err != nil {
		return time.Time{}, err
	}
	m, err := strconv.Atoi(month)
	if err != nil {
		return time.Time{}, err
	}
	d, err := strconv.Atoi(day)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), nil
}
