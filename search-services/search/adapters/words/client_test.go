package words

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	wordspb "yadro.com/course/proto/words"
)

type mockWordsServer struct {
	wordspb.UnimplementedWordsServer
	normFunc func(context.Context, *wordspb.WordsRequest) (*wordspb.WordsReply, error)
}

func (m *mockWordsServer) Norm(ctx context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	return m.normFunc(ctx, req)
}

func TestClient_Norm(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	s := grpc.NewServer()
	mockServer := &mockWordsServer{}
	wordspb.RegisterWordsServer(s, mockServer)

	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	conn, err := grpc.NewClient("passthrough://buf",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "failed to create client connection")
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}()

	client := &Client{
		log:    slog.Default(),
		conn:   conn,
		client: wordspb.NewWordsClient(conn),
	}

	t.Run("Success", func(t *testing.T) {
		expected := []string{"apple", "banana"}
		mockServer.normFunc = func(ctx context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
			assert.Equal(t, "apples bananas", req.Phrase, "phrase mismatch")
			return &wordspb.WordsReply{Words: expected}, nil
		}

		res, err := client.Norm(context.Background(), "apples bananas")
		assert.NoError(t, err, "Norm failed")
		assert.Equal(t, expected, res, "response mismatch")
	})

	t.Run("ServerError", func(t *testing.T) {
		mockServer.normFunc = func(ctx context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
			return nil, errors.New("grpc internal error")
		}

		res, err := client.Norm(context.Background(), "test")
		assert.Error(t, err, "expected error")
		assert.Nil(t, res, "result should be nil on error")
	})
}

func TestNewClient(t *testing.T) {
	logger := slog.Default()

	t.Run("Success", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err, "failed to listen")
		defer func() {
			if err := lis.Close(); err != nil {
				t.Logf("failed to close listener: %v", err)
			}
		}()

		s := grpc.NewServer()
		mockServer := &mockWordsServer{
			normFunc: func(ctx context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
				return &wordspb.WordsReply{Words: []string{"hello"}}, nil
			},
		}
		wordspb.RegisterWordsServer(s, mockServer)
		go func() {
			_ = s.Serve(lis)
		}()
		defer s.Stop()

		addr := lis.Addr().String()
		client, err := NewClient(addr, logger)
		require.NoError(t, err, "NewClient failed")
		assert.NotNil(t, client)

		words, err := client.Norm(context.Background(), "test")
		assert.NoError(t, err, "Norm call failed")
		assert.Equal(t, []string{"hello"}, words, "unexpected normalization result")

		err = client.Close()
		assert.NoError(t, err, "Close failed")
	})

	t.Run("InvalidAddress", func(t *testing.T) {
		_, err := NewClient("", logger)
		if err == nil {
			t.Skip("grpc.NewClient does not validate address eagerly")
		}
		assert.Error(t, err, "expected error for invalid address")
	})
}

func TestClient_Close(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to listen")
	defer func() {
		if err := lis.Close(); err != nil {
			t.Logf("failed to close listener: %v", err)
		}
	}()

	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, &mockWordsServer{
		normFunc: func(ctx context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
			return &wordspb.WordsReply{}, nil
		},
	})
	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	client, err := NewClient(lis.Addr().String(), slog.Default())
	require.NoError(t, err, "NewClient failed")

	err = client.Close()
	assert.NoError(t, err, "first Close failed")

	_ = client.Close()
}
