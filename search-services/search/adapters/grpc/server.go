package grpc

import (
	"context"
	"errors"
	"log/slog"

	searchpb "yadro.com/course/proto/search"
	"yadro.com/course/search/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	searchpb.UnimplementedSearchServer
	service core.Searcher
	logger  *slog.Logger
}

func NewServer(service core.Searcher, logger *slog.Logger) *Server {
	return &Server{service: service, logger: logger}
}

func (s *Server) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Server) Search(ctx context.Context, req *searchpb.SearchRequest) (*searchpb.SearchResponse, error) {
	phrase := req.GetPhrase()
	limit := req.GetLimit()

	if limit <= 0 {
		limit = 10
	}

	comics, total, err := s.service.Search(ctx, phrase, int(limit))
	if err != nil {
		return nil, s.handleError(err, "search failed")
	}

	return s.prepareResponse(comics, total), nil
}

func (s *Server) ISearch(ctx context.Context, req *searchpb.SearchRequest) (*searchpb.SearchResponse, error) {
	phrase := req.GetPhrase()
	limit := req.GetLimit()

	if limit <= 0 {
		limit = 10
	}

	comics, total, err := s.service.ISearch(ctx, phrase, int(limit))
	if err != nil {
		return nil, s.handleError(err, "index search failed")
	}

	return s.prepareResponse(comics, total), nil
}

func (s *Server) handleError(err error, msg string) error {
	if errors.Is(err, core.ErrEmptyPhrase) {
		return status.Error(codes.InvalidArgument, "empty phrase")
	}
	s.logger.Error(msg, "error", err)
	return status.Errorf(codes.Internal, "%s: %v", msg, err)
}

func (s *Server) prepareResponse(comics []core.Comics, total int) *searchpb.SearchResponse {
	pbComics := make([]*searchpb.Comics, 0, len(comics))
	for _, c := range comics {
		pbComics = append(pbComics, &searchpb.Comics{
			Id:  int32(c.Num),
			Url: c.ImgURL,
		})
	}

	return &searchpb.SearchResponse{
		Comics: pbComics,
		Total:  int32(total),
	}
}
