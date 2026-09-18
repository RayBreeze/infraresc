package aws

import (
	"context"
	"fmt"
)

type Collector struct {
	discovery *Discovery
}

func NewCollector(client *Client) *Collector {
	return &Collector{
		discovery: NewDiscovery(client),
	}
}

func (c *Collector) Collect(
	ctx context.Context,
) ([]string, error) {

	if err := c.discovery.CheckRecorderConfiguration(ctx); err != nil {
		return nil, err
	}

	if err := c.discovery.CheckRecorder(ctx); err != nil {
		return nil, err
	}

	resources, err := c.discovery.Resources(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"collecting AWS resources: %w",
			err,
		)
	}

	return resources, nil
}
