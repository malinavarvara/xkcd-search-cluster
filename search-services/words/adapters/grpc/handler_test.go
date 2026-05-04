package grpc

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "yadro.com/course/proto/words"
)

type mockNorm struct{}

func (m *mockNorm) Normalize(phrase string) []string { return []string{"test"} }

func TestServer_Norm(t *testing.T) {
	srv := NewServer(&mockNorm{})

	t.Run("Success", func(t *testing.T) {
		req := &wordspb.WordsRequest{Phrase: "hello"}
		res, err := srv.Norm(context.Background(), req)
		if err != nil || len(res.GetWords()) == 0 {
			t.Errorf("expected success, got err: %v", err)
		}
	})

	t.Run("Too_Large_Payload", func(t *testing.T) {
		largePhrase := strings.Repeat("a", maxPhraseLen+1)
		req := &wordspb.WordsRequest{Phrase: largePhrase}

		_, err := srv.Norm(context.Background(), req)
		if status.Code(err) != codes.ResourceExhausted {
			t.Errorf("expected ResourceExhausted, got %v", status.Code(err))
		}
	})

	t.Run("Deadline_Exceeded", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := &wordspb.WordsRequest{Phrase: "test"}
		_, err := srv.Norm(ctx, req)
		if status.Code(err) != codes.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", status.Code(err))
		}
	})

	t.Run("Ping", func(t *testing.T) {
		_, err := srv.Ping(context.Background(), &emptypb.Empty{})
		if err != nil {
			t.Error(err)
		}
	})
}
