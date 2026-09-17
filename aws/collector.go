package aws

import (
	"context"

	"infraresc/state"
)

type Collector struct {
	discovery *Discovery
}

func NewCollector(client *Client) *Collector {
	return &Collector{
		discovery: NewDiscovery(client),
	}
}

func (c *Collector) Collect(ctx context.Context) ([]state.Resource, error) {
	return c.discovery.Discover(ctx)
}
