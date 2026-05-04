package update

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
	updatepb "yadro.com/course/proto/update"
)

type MockUpdateClient struct {
	mock.Mock
}

func (m *MockUpdateClient) Ping(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	args := m.Called(ctx, in)
	return &emptypb.Empty{}, args.Error(1)
}

func (m *MockUpdateClient) Status(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*updatepb.StatusReply, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*updatepb.StatusReply), args.Error(1)
}

func (m *MockUpdateClient) Stats(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*updatepb.StatsReply, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*updatepb.StatsReply), args.Error(1)
}

func (m *MockUpdateClient) Update(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	args := m.Called(ctx, in)
	return &emptypb.Empty{}, args.Error(1)
}

func (m *MockUpdateClient) Drop(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	args := m.Called(ctx, in)
	return &emptypb.Empty{}, args.Error(1)
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

func TestClient_Methods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Ping", func(t *testing.T) {
		mockRPC := new(MockUpdateClient)
		client := &Client{client: mockRPC, log: logger}
		mockRPC.On("Ping", mock.Anything, mock.Anything).Return(&emptypb.Empty{}, nil).Once()

		err := client.Ping(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Status_Mapping", func(t *testing.T) {
		mockRPC := new(MockUpdateClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("Status", mock.Anything, mock.Anything).
			Return(&updatepb.StatusReply{Status: updatepb.Status_STATUS_IDLE}, nil).Once()
		res, err := client.Status(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, core.StatusUpdateIdle, res)

		mockRPC.On("Status", mock.Anything, mock.Anything).
			Return(&updatepb.StatusReply{Status: updatepb.Status_STATUS_RUNNING}, nil).Once()
		res, err = client.Status(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, core.StatusUpdateRunning, res)

		mockRPC.On("Status", mock.Anything, mock.Anything).
			Return(nil, errors.New("rpc error")).Once()
		res, err = client.Status(context.Background())
		assert.Error(t, err)
		assert.Equal(t, core.StatusUpdateUnknown, res)

		mockRPC.On("Status", mock.Anything, mock.Anything).
			Return(&updatepb.StatusReply{Status: 999}, nil).Once()
		res, err = client.Status(context.Background())
		assert.Error(t, err)
		assert.Equal(t, core.StatusUpdateUnknown, res)
	})

	t.Run("Stats_Mapping", func(t *testing.T) {
		mockRPC := new(MockUpdateClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("Stats", mock.Anything, mock.Anything).Return(&updatepb.StatsReply{
			WordsTotal:    100,
			WordsUnique:   50,
			ComicsFetched: 10,
			ComicsTotal:   200,
		}, nil).Once()

		res, err := client.Stats(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 100, res.WordsTotal)
		assert.Equal(t, 200, res.ComicsTotal)

		mockRPC.On("Stats", mock.Anything, mock.Anything).Return(nil, errors.New("rpc error")).Once()
		_, err = client.Stats(context.Background())
		assert.Error(t, err)
	})

	t.Run("Update", func(t *testing.T) {
		mockRPC := new(MockUpdateClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("Update", mock.Anything, mock.Anything).Return(&emptypb.Empty{}, nil).Once()
		err := client.Update(context.Background())
		assert.NoError(t, err)

		mockRPC.On("Update", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.AlreadyExists, "running")).Once()
		err = client.Update(context.Background())
		assert.ErrorIs(t, err, core.ErrUpdateAlreadyRunning)

		mockRPC.On("Update", mock.Anything, mock.Anything).
			Return(nil, errors.New("other error")).Once()
		err = client.Update(context.Background())
		assert.Error(t, err)
	})

	t.Run("Drop", func(t *testing.T) {
		mockRPC := new(MockUpdateClient)
		client := &Client{client: mockRPC, log: logger}

		mockRPC.On("Drop", mock.Anything, mock.Anything).Return(&emptypb.Empty{}, nil).Once()
		err := client.Drop(context.Background())
		assert.NoError(t, err)

		mockRPC.On("Drop", mock.Anything, mock.Anything).Return(nil, errors.New("drop fail")).Once()
		err = client.Drop(context.Background())
		assert.Error(t, err)
		//
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
