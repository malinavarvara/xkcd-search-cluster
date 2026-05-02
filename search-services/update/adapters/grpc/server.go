package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	updatepb "yadro.com/course/proto/update"
	"yadro.com/course/update/core"
)

func NewServer(service core.Updater) *Server {
	return &Server{service: service}
}

type Server struct {
	updatepb.UnimplementedUpdateServer
	service core.Updater
}

func (s *Server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Server) Status(ctx context.Context, _ *emptypb.Empty) (*updatepb.StatusReply, error) {
	statusVal := s.service.Status(ctx)
	var pbStatus updatepb.Status
	switch statusVal {
	case core.StatusRunning:
		pbStatus = updatepb.Status_STATUS_RUNNING
	case core.StatusIdle:
		pbStatus = updatepb.Status_STATUS_IDLE
	default:
		return nil, status.Errorf(codes.Internal, "unknown status: %s", statusVal)
	}
	return &updatepb.StatusReply{Status: pbStatus}, nil
}

func (s *Server) Update(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.service.Update(ctx)
	if err != nil {
		if errors.Is(err, core.ErrUpdateAlreadyRunning) {
			return nil, status.Error(codes.AlreadyExists, "update already running")
		}
		return nil, status.Errorf(codes.Internal, "failed to start update: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) Stats(ctx context.Context, _ *emptypb.Empty) (*updatepb.StatsReply, error) {
	stats, err := s.service.Stats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stats: %v", err)
	}
	return &updatepb.StatsReply{
		WordsTotal:    int64(stats.WordsTotal),
		WordsUnique:   int64(stats.WordsUnique),
		ComicsFetched: int64(stats.ComicsFetched),
		ComicsTotal:   int64(stats.ComicsTotal),
	}, nil
}

func (s *Server) Drop(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.service.Drop(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "drop failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}
