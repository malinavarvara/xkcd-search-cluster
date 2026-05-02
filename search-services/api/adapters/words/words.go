package words

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"yadro.com/course/api/core"
	wordspb "yadro.com/course/proto/words"
)

type Client struct {
	log    *slog.Logger
	client wordspb.WordsClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	return &Client{
		log:    log,
		client: wordspb.NewWordsClient(conn),
		conn:   conn,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Norm(ctx context.Context, phrase string) ([]string, error) {
	resp, err := c.client.Norm(ctx, &wordspb.WordsRequest{Phrase: phrase})
	if err != nil {
		c.log.Error("gRPC call Norm failed", "phrase", phrase, "error", err)
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.ResourceExhausted:
				return nil, core.ErrPhraseTooLarge
			case codes.DeadlineExceeded:
				return nil, core.ErrRequestTimeout
			case codes.Unavailable:
				return nil, core.ErrServiceUnavailable
			case codes.InvalidArgument:
				return nil, core.ErrInvalidArgument
			default:
				return nil, err
			}
		}
		return nil, err
	}
	return resp.Words, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	if err != nil {
		c.log.Debug("ping failed", "error", err)
	}
	return err
}
