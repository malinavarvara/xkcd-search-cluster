package core

import "context"

type Normalizer interface {
	Norm(context.Context, string) ([]string, error)
}

type Pinger interface {
	Ping(context.Context) error
}

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

type UpdateClient interface {
	Ping(ctx context.Context) error
	Status(ctx context.Context) (UpdateStatus, error)
	Stats(ctx context.Context) (UpdateStats, error)
	Update(ctx context.Context) error
	Drop(ctx context.Context) error
}

type Searcher interface {
	Search(ctx context.Context, phrase string, limit int) ([]Comics, int, error)
	ISearch(ctx context.Context, phrase string, limit int) ([]Comics, int, error)
	BuildIndex(ctx context.Context) error
}

type Authenticator interface {
	Login(name, password string) (string, error)
	Verify(token string) error
}
