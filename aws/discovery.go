package aws

import (
	"context"

	"infraresc/state"
)

type Discovery struct {
	client *Client
}

func NewDiscovery(client *Client) *Discovery {
	return &Discovery{
		client: client,
	}
}

func (d *Discovery) Discover(ctx context.Context) ([]state.Resource, error) {
	return []state.Resource{}, nil
}
