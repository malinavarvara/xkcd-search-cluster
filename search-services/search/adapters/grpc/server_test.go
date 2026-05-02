package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	searchpb "yadro.com/course/proto/search"
	"yadro.com/course/search/core"
)

type MockSearcher struct {
	core.Searcher
	searchFunc  func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error)
	iSearchFunc func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error)
}

func (m *MockSearcher) Search(ctx context.Context, p string, l int) ([]core.Comics, int, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, p, l)
	}
	return nil, 0, nil
}

func (m *MockSearcher) ISearch(ctx context.Context, p string, l int) ([]core.Comics, int, error) {
	if m.iSearchFunc != nil {
		return m.iSearchFunc(ctx, p, l)
	}
	return nil, 0, nil
}

func TestServer_Search(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Success", func(t *testing.T) {
		mock := &MockSearcher{
			searchFunc: func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
				assert.Equal(t, "funny comic", phrase)
				assert.Equal(t, 5, limit)
				return []core.Comics{
					{Num: 1, ImgURL: "http://xkcd.com/1.png"},
				}, 1, nil
			},
		}
		srv := NewServer(mock, logger)
		req := &searchpb.SearchRequest{Phrase: "funny comic", Limit: 5}

		resp, err := srv.Search(context.Background(), req)
		require.NoError(t, err, "Search failed")

		assert.Equal(t, int32(1), resp.Total, "total mismatch")
		assert.Len(t, resp.Comics, 1, "comics count mismatch")
		assert.Equal(t, int32(1), resp.Comics[0].Id, "comic ID mismatch")
		assert.Equal(t, "http://xkcd.com/1.png", resp.Comics[0].Url, "comic URL mismatch")
	})

	t.Run("EmptyPhraseError", func(t *testing.T) {
		mock := &MockSearcher{
			searchFunc: func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
				return nil, 0, core.ErrEmptyPhrase
			},
		}
		srv := NewServer(mock, logger)
		_, err := srv.Search(context.Background(), &searchpb.SearchRequest{Phrase: ""})

		st, ok := status.FromError(err)
		assert.True(t, ok, "expected status error")
		assert.Equal(t, codes.InvalidArgument, st.Code(), "expected InvalidArgument code")
	})

	t.Run("InternalError", func(t *testing.T) {
		mock := &MockSearcher{
			searchFunc: func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
				return nil, 0, errors.New("database connection failed")
			},
		}
		srv := NewServer(mock, logger)
		_, err := srv.Search(context.Background(), &searchpb.SearchRequest{Phrase: "test"})

		st, ok := status.FromError(err)
		assert.True(t, ok, "expected status error")
		assert.Equal(t, codes.Internal, st.Code(), "expected Internal code")
	})
}

func TestServer_Ping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(&MockSearcher{}, logger)

	resp, err := srv.Ping(context.Background(), &emptypb.Empty{})
	assert.NoError(t, err, "Ping should not error")
	assert.NotNil(t, resp, "response should not be nil")
}

func TestServer_ISearch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("UsesDefaultLimit", func(t *testing.T) {
		mock := &MockSearcher{
			iSearchFunc: func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
				assert.Equal(t, 10, limit, "default limit should be 10")
				return []core.Comics{}, 0, nil
			},
		}
		srv := NewServer(mock, logger)
		_, err := srv.ISearch(context.Background(), &searchpb.SearchRequest{Phrase: "test", Limit: 0})
		assert.NoError(t, err, "ISearch should not error")
	})

	t.Run("EmptyPhraseError", func(t *testing.T) {
		mock := &MockSearcher{
			iSearchFunc: func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
				return nil, 0, core.ErrEmptyPhrase
			},
		}
		srv := NewServer(mock, logger)
		_, err := srv.ISearch(context.Background(), &searchpb.SearchRequest{Phrase: ""})
		st, ok := status.FromError(err)
		assert.True(t, ok, "expected status error")
		assert.Equal(t, codes.InvalidArgument, st.Code(), "expected InvalidArgument code")
	})

	t.Run("InternalError", func(t *testing.T) {
		mock := &MockSearcher{
			iSearchFunc: func(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
				return nil, 0, errors.New("index search failed")
			},
		}
		srv := NewServer(mock, logger)
		_, err := srv.ISearch(context.Background(), &searchpb.SearchRequest{Phrase: "test"})
		st, ok := status.FromError(err)
		assert.True(t, ok, "expected status error")
		assert.Equal(t, codes.Internal, st.Code(), "expected Internal code")
	})
}
