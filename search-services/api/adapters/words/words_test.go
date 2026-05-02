package words

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"yadro.com/course/api/core"
	wordspb "yadro.com/course/proto/words"
)

type MockWordsClient struct {
	mock.Mock
}

func (m *MockWordsClient) Norm(ctx context.Context, in *wordspb.WordsRequest, opts ...grpc.CallOption) (*wordspb.WordsReply, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*wordspb.WordsReply), args.Error(1)
}

func (m *MockWordsClient) Ping(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
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
		defer func() {
			err := client.Close()
			if err != nil {
				t.Logf("failed to close client: %v", err)
			}
		}()
	})
}

func TestClient_Close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, _ := NewClient("localhost:50051", logger)

	t.Run("Close_Success", func(t *testing.T) {
		err := client.Close()
		assert.NoError(t, err)
	})
}

func TestClient_Norm(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Success", func(t *testing.T) {
		mockRPC := new(MockWordsClient)
		client := &Client{
			log:    logger,
			client: mockRPC,
		}

		expectedWords := []string{"hello", "world"}
		mockRPC.On("Norm", mock.Anything, &wordspb.WordsRequest{Phrase: "test"}).
			Return(&wordspb.WordsReply{Words: expectedWords}, nil)

		words, err := client.Norm(context.Background(), "test")

		assert.NoError(t, err)
		assert.Equal(t, expectedWords, words)
	})

	t.Run("MappingErrors", func(t *testing.T) {
		tests := []struct {
			name     string
			grpcErr  error
			expected error
		}{
			{
				name:     "ResourceExhausted",
				grpcErr:  status.Error(codes.ResourceExhausted, "too big"),
				expected: core.ErrPhraseTooLarge,
			},
			{
				name:     "DeadlineExceeded",
				grpcErr:  status.Error(codes.DeadlineExceeded, "timeout"),
				expected: core.ErrRequestTimeout,
			},
			{
				name:     "Unavailable",
				grpcErr:  status.Error(codes.Unavailable, "down"),
				expected: core.ErrServiceUnavailable,
			},
			{
				name:     "InvalidArgument",
				grpcErr:  status.Error(codes.InvalidArgument, "bad req"),
				expected: core.ErrInvalidArgument,
			},
			{
				name:     "Unknown_gRPC_Error",
				grpcErr:  status.Error(codes.Internal, "internal"),
				expected: status.Error(codes.Internal, "internal"),
			},
			{
				name:     "Generic_Error",
				grpcErr:  errors.New("generic error"),
				expected: errors.New("generic error"),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockRPC := new(MockWordsClient)
				client := &Client{log: logger, client: mockRPC}

				mockRPC.On("Norm", mock.Anything, mock.Anything).Return(nil, tt.grpcErr)

				_, err := client.Norm(context.Background(), "test")
				if tt.name == "Generic_Error" || tt.name == "Unknown_gRPC_Error" {
					assert.Equal(t, tt.expected.Error(), err.Error())
				} else {
					assert.ErrorIs(t, err, tt.expected)
				}
			})
		}
	})
}

func TestClient_Ping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Ping_Success", func(t *testing.T) {
		mockRPC := new(MockWordsClient)
		client := &Client{log: logger, client: mockRPC}

		mockRPC.On("Ping", mock.Anything, mock.Anything).Return(&emptypb.Empty{}, nil)

		err := client.Ping(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Ping_Failure", func(t *testing.T) {
		mockRPC := new(MockWordsClient)
		client := &Client{log: logger, client: mockRPC}

		mockRPC.On("Ping", mock.Anything, mock.Anything).Return(nil, errors.New("connection lost"))

		err := client.Ping(context.Background())
		assert.Error(t, err)
		assert.Equal(t, "connection lost", err.Error())
	})
}
