package update

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
	updatepb "yadro.com/course/proto/update"
)

type Client struct {
	log    *slog.Logger
	client updatepb.UpdateClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}
	return &Client{
		client: updatepb.NewUpdateClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Status(ctx context.Context) (core.UpdateStatus, error) {
	resp, err := c.client.Status(ctx, &emptypb.Empty{})
	if err != nil {
		return core.StatusUpdateUnknown, fmt.Errorf("status request failed: %w", err)
	}
	switch resp.Status {
	case updatepb.Status_STATUS_IDLE:
		return core.StatusUpdateIdle, nil
	case updatepb.Status_STATUS_RUNNING:
		return core.StatusUpdateRunning, nil
	default:
		return core.StatusUpdateUnknown, fmt.Errorf("unknown status from update service: %v", resp.Status)
	}
}

func (c *Client) Stats(ctx context.Context) (core.UpdateStats, error) {
	resp, err := c.client.Stats(ctx, &emptypb.Empty{})
	if err != nil {
		return core.UpdateStats{}, fmt.Errorf("stats request failed: %w", err)
	}
	return core.UpdateStats{
		WordsTotal:    int(resp.WordsTotal),
		WordsUnique:   int(resp.WordsUnique),
		ComicsFetched: int(resp.ComicsFetched),
		ComicsTotal:   int(resp.ComicsTotal),
	}, nil
}

func (c *Client) Update(ctx context.Context) error {
	_, err := c.client.Update(ctx, &emptypb.Empty{})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			return core.ErrUpdateAlreadyRunning
		}
		return err
	}
	return nil
}

func (c *Client) Drop(ctx context.Context) error {
	_, err := c.client.Drop(ctx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("drop request failed: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
