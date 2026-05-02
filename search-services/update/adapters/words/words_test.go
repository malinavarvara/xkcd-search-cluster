package words

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "yadro.com/course/proto/words"
)

type mockWordsServer struct {
	wordspb.UnimplementedWordsServer
	err error
}

func (m *mockWordsServer) Norm(_ context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &wordspb.WordsReply{Words: []string{"normalized"}}, nil
}

func (m *mockWordsServer) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, m.err
}

func TestClient(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	mockSrv := &mockWordsServer{}
	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, mockSrv)

	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create client")
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}()

	client := &Client{
		conn:   conn,
		client: wordspb.NewWordsClient(conn),
		log:    slog.Default(),
	}

	t.Run("Norm_Success", func(t *testing.T) {
		words, err := client.Norm(context.Background(), "test phrase")
		assert.NoError(t, err)
		assert.Equal(t, []string{"normalized"}, words)
	})

	t.Run("Norm_Error", func(t *testing.T) {
		mockSrv.err = fmt.Errorf("grpc error")
		defer func() { mockSrv.err = nil }()

		_, err := client.Norm(context.Background(), "test")
		assert.Error(t, err)
	})

	t.Run("Ping_Success", func(t *testing.T) {
		err := client.Ping(context.Background())
		assert.NoError(t, err)
	})

	t.Run("NewClient_Error", func(t *testing.T) {
		_, err := NewClient("invalid-address", slog.Default())
		assert.NoError(t, err, "did not expect error on creation")
	})

	t.Run("Close_Success", func(t *testing.T) {
		conn, err := grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		client := &Client{
			conn:   conn,
			client: wordspb.NewWordsClient(conn),
			log:    slog.Default(),
		}

		err = client.Close()
		assert.NoError(t, err)

		state := conn.GetState()
		assert.NotEqual(t, connectivity.Ready, state, "connection should not be Ready after Close")
	})

	t.Run("Norm_After_Close_Error", func(t *testing.T) {
		conn, err := grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		client := &Client{
			conn:   conn,
			client: wordspb.NewWordsClient(conn),
			log:    slog.Default(),
		}
		err = client.Close()
		require.NoError(t, err)

		_, err = client.Norm(context.Background(), "test")
		assert.Error(t, err, "expected error when calling Norm after Close")
	})
}
