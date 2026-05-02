package search

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	searchpb "yadro.com/course/proto/search"
)

type MockSearchGrpcClient struct {
	mock.Mock
}

func (m *MockSearchGrpcClient) Search(ctx context.Context, in *searchpb.SearchRequest, opts ...grpc.CallOption) (*searchpb.SearchResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*searchpb.SearchResponse), args.Error(1)
}

func (m *MockSearchGrpcClient) ISearch(ctx context.Context, in *searchpb.SearchRequest, opts ...grpc.CallOption) (*searchpb.SearchResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*searchpb.SearchResponse), args.Error(1)
}

func (m *MockSearchGrpcClient) Ping(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*emptypb.Empty), args.Error(1)
}

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Success", func(t *testing.T) {
		client, err := NewClient("localhost:50051", logger)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		err = client.Close()
		require.NoError(t, err)
	})
}

func TestClient_Search(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Success Mapping", func(t *testing.T) {
		mockRPC := new(MockSearchGrpcClient)
		client := &Client{client: mockRPC, log: logger}

		expectedPhrase := "test query"
		expectedLimit := 5

		mockRPC.On("Search", mock.Anything, &searchpb.SearchRequest{
			Phrase: expectedPhrase,
			Limit:  int32(expectedLimit),
		}).Return(&searchpb.SearchResponse{
			Comics: []*searchpb.Comics{
				{Id: 100, Url: "http://xkcd.com/100/info.0.json"},
			},
			Total: 1,
		}, nil)

		comics, total, err := client.Search(context.Background(), expectedPhrase, expectedLimit)

		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, comics, 1)
		assert.Equal(t, 100, comics[0].ID)
		assert.Equal(t, "http://xkcd.com/100/info.0.json", comics[0].ImgURL)
	})

	t.Run("RPC Error", func(t *testing.T) {
		mockRPC := new(MockSearchGrpcClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("Search", mock.Anything, mock.Anything).
			Return(nil, errors.New("connection refused"))

		_, _, err := client.Search(context.Background(), "fail", 10)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "grpc search request failed")
	})
}

func TestClient_ISearch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Success", func(t *testing.T) {
		mockRPC := new(MockSearchGrpcClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("ISearch", mock.Anything, mock.Anything).Return(&searchpb.SearchResponse{
			Comics: []*searchpb.Comics{{Id: 1, Url: "url"}},
			Total:  1,
		}, nil)

		comics, total, err := client.ISearch(context.Background(), "phrase", 10)

		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, 1, comics[0].ID)
	})

	t.Run("Error", func(t *testing.T) {
		mockRPC := new(MockSearchGrpcClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("ISearch", mock.Anything, mock.Anything).
			Return(nil, errors.New("isearch failed"))

		_, _, err := client.ISearch(context.Background(), "phrase", 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "grpc isearch request failed")
	})
}

func TestClient_BuildIndex(t *testing.T) {
	client := &Client{}
	t.Run("Success", func(t *testing.T) {
		err := client.BuildIndex(context.Background())
		assert.NoError(t, err)
	})
}

func TestClient_Ping(t *testing.T) {
	mockRPC := new(MockSearchGrpcClient)
	client := &Client{client: mockRPC}

	mockRPC.On("Ping", mock.Anything, mock.Anything).Return(&emptypb.Empty{}, nil)

	err := client.Ping(context.Background())
	assert.NoError(t, err)
}

func TestClient_Close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, _ := NewClient("localhost:50051", logger)

	t.Run("Close_Success", func(t *testing.T) {
		err := client.Close()
		assert.NoError(t, err)
	})
}
