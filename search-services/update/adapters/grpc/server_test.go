package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	updatepb "yadro.com/course/proto/update"
	"yadro.com/course/update/core"
)

type mockService struct {
	status core.ServiceStatus
	stats  core.ServiceStats
	err    error
}

func (m *mockService) Update(_ context.Context) error {
	return m.err
}

func (m *mockService) Stats(_ context.Context) (core.ServiceStats, error) {
	return m.stats, m.err
}

func (m *mockService) Status(_ context.Context) core.ServiceStatus {
	return m.status
}

func (m *mockService) Drop(_ context.Context) error {
	return m.err
}

func TestServer(t *testing.T) {
	svc := &mockService{}
	server := NewServer(svc)

	t.Run("Ping_Success", func(t *testing.T) {
		_, err := server.Ping(context.Background(), &emptypb.Empty{})
		assert.NoError(t, err, "Ping failed")
	})

	t.Run("Status_Running", func(t *testing.T) {
		svc.status = core.StatusRunning
		resp, err := server.Status(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		assert.Equal(t, updatepb.Status_STATUS_RUNNING, resp.Status, "expected STATUS_RUNNING")
	})

	t.Run("Status_Idle", func(t *testing.T) {
		svc.status = core.StatusIdle
		resp, err := server.Status(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		assert.Equal(t, updatepb.Status_STATUS_IDLE, resp.Status, "expected STATUS_IDLE")
	})

	t.Run("Status_Unknown_InternalError", func(t *testing.T) {
		svc.status = "some-random-string"
		_, err := server.Status(context.Background(), &emptypb.Empty{})
		assert.Equal(t, codes.Internal, status.Code(err), "expected Internal for unknown status")
	})

	t.Run("Update_Success", func(t *testing.T) {
		svc.err = nil
		_, err := server.Update(context.Background(), &emptypb.Empty{})
		assert.NoError(t, err, "Update failed")
	})

	t.Run("Update_Conflict_AlreadyRunning", func(t *testing.T) {
		svc.err = core.ErrUpdateAlreadyRunning
		_, err := server.Update(context.Background(), &emptypb.Empty{})
		assert.Equal(t, codes.AlreadyExists, status.Code(err), "expected AlreadyExists for already running update")
	})

	t.Run("Stats_Success", func(t *testing.T) {
		svc.err = nil
		svc.stats = core.ServiceStats{
			DBStats: core.DBStats{
				WordsTotal:    500,
				WordsUnique:   100,
				ComicsFetched: 10,
			},
			ComicsTotal: 50,
		}

		resp, err := server.Stats(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		assert.Equal(t, int64(500), resp.WordsTotal, "WordsTotal mismatch")
		assert.Equal(t, int64(50), resp.ComicsTotal, "ComicsTotal mismatch")
	})

	t.Run("Stats_Error", func(t *testing.T) {
		svc.err = errors.New("database failure")
		_, err := server.Stats(context.Background(), &emptypb.Empty{})
		assert.Equal(t, codes.Internal, status.Code(err), "expected Internal error on stats failure")
	})

	t.Run("Drop_Success", func(t *testing.T) {
		svc.err = nil
		_, err := server.Drop(context.Background(), &emptypb.Empty{})
		assert.NoError(t, err, "Drop failed")
	})

	t.Run("Update_InternalError", func(t *testing.T) {
		svc.err = errors.New("disk full")
		_, err := server.Update(context.Background(), &emptypb.Empty{})
		assert.Equal(t, codes.Internal, status.Code(err), "expected Internal error on update failure")
	})

	t.Run("Drop_InternalError", func(t *testing.T) {
		svc.err = errors.New("something went wrong")
		_, err := server.Drop(context.Background(), &emptypb.Empty{})
		assert.Equal(t, codes.Internal, status.Code(err), "expected Internal error on drop failure")
	})
}
