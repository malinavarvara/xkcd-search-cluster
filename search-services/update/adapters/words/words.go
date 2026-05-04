package words

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "yadro.com/course/proto/words"
)

type Client struct {
	log    *slog.Logger
	conn   *grpc.ClientConn
	client wordspb.WordsClient
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to words service: %w", err)
	}
	return &Client{
		conn:   conn,
		client: wordspb.NewWordsClient(conn),
		log:    log,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Norm(ctx context.Context, phrase string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req := &wordspb.WordsRequest{Phrase: phrase}
	resp, err := c.client.Norm(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("normalize request: %w", err)
	}
	return resp.Words, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}
