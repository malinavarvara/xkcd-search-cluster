package core

import (
	"context"
	"time"
)

type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}

type Searcher interface {
	Search(ctx context.Context, phrase string, limit int) ([]Comics, int, error)
	ISearch(ctx context.Context, phrase string, limit int) ([]Comics, int, error)
	BuildIndex(ctx context.Context) error
	TriggerRebuild()
	RunBackgroundIndexer(ctx context.Context, interval time.Duration)
}

type ComicsRepository interface {
	SearchComics(ctx context.Context, words []string, limit int) ([]Comics, error)
	CountSearchResults(ctx context.Context, words []string) (int, error)
	GetAllComicsWords(ctx context.Context) (map[string][]int, error)
	GetComicsByIDs(ctx context.Context, ids []int) ([]Comics, error)
}
