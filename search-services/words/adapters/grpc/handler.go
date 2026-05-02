package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	wordspb "yadro.com/course/proto/words"
	"yadro.com/course/words/core"
)

const maxPhraseLen = 16 * 1024

type Server struct {
	wordspb.UnimplementedWordsServer
	normalizer core.Normalizer
}

func NewServer(normalizer core.Normalizer) *Server {
	return &Server{normalizer: normalizer}
}

func (s *Server) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Server) Norm(ctx context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	if len(req.GetPhrase()) > maxPhraseLen {
		return nil, status.Error(codes.ResourceExhausted, "message exceeds 16 Kb limit")
	}

	if err := ctx.Err(); err != nil {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	normalized := s.normalizer.Normalize(req.GetPhrase())

	return &wordspb.WordsReply{Words: normalized}, nil
}
