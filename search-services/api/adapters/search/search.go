package search

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/protobuf/types/known/emptypb"
	"yadro.com/course/api/core"
	searchpb "yadro.com/course/proto/search"
)

type Client struct {
	log    *slog.Logger
	conn   *grpc.ClientConn
	client searchpb.SearchClient
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}
	return &Client{
		client: searchpb.NewSearchClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c *Client) Search(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
	resp, err := c.client.Search(ctx, &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("grpc search request failed: %w", err)
	}

	pbComics := resp.GetComics()
	comics := make([]core.Comics, 0, len(pbComics))

	for _, pb := range pbComics {
		comics = append(comics, core.Comics{
			ID:     int(pb.GetId()),
			ImgURL: pb.GetUrl(),
		})
	}

	return comics, int(resp.GetTotal()), nil
}

func (c *Client) ISearch(ctx context.Context, phrase string, limit int) ([]core.Comics, int, error) {
	resp, err := c.client.ISearch(ctx, &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("grpc isearch request failed: %w", err)
	}

	pbComics := resp.GetComics()
	comics := make([]core.Comics, 0, len(pbComics))
	for _, pb := range pbComics {
		comics = append(comics, core.Comics{
			ID:     int(pb.GetId()),
			ImgURL: pb.GetUrl(),
		})
	}

	return comics, int(resp.GetTotal()), nil
}

func (c *Client) BuildIndex(ctx context.Context) error {
	return nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
