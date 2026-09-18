package aws

import (
	"context"
	"fmt"

	"infraresc/state"
)

type Collector struct {
	discovery     *Discovery
	relationships *RelationshipDiscovery
}

func NewCollector(client *Client) *Collector {
	return &Collector{
		discovery:     NewDiscovery(client),
		relationships: NewRelationshipDiscovery(client),
	}
}

func (c *Collector) Collect(
	ctx context.Context,
) (*state.Infrastructure, error) {

	resources, err := c.discovery.Resources(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"discovering resources: %w",
			err,
		)
	}

	edges, err := c.relationships.Discover(
		ctx,
		resources,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"discovering relationships: %w",
			err,
		)
	}

	return &state.Infrastructure{
		Resources: resources,
		Edges:     edges,
	}, nil
}
