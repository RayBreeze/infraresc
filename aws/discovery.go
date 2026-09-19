package aws

import (
	"context"
	"fmt"
	"strings"

	"infraresc/state"

	"github.com/aws/aws-sdk-go-v2/service/resourceexplorer2"
)

type Discovery struct {
	client *resourceexplorer2.Client
}

func NewDiscovery(client *Client) *Discovery {
	return &Discovery{
		client: client.ResourceExplorer,
	}
}

func (d *Discovery) Resources(
	ctx context.Context,
) ([]state.Resource, error) {

	var resources []state.Resource

	paginator := resourceexplorer2.NewListResourcesPaginator(
		d.client,
		&resourceexplorer2.ListResourcesInput{},
	)

	for paginator.HasMorePages() {

		page, err := paginator.NextPage(ctx)

		if err != nil {
			return nil, fmt.Errorf(
				"resource discovery failed: %w",
				err,
			)
		}

		for _, resource := range page.Resources {

			if resource.Arn == nil {
				continue
			}

			resourceType := stringValue(
				resource.CfnResourceType,
			)

			if resourceType == "" {
				resourceType = stringValue(
					resource.ResourceType,
				)
			}

			if resourceType == "" {
				continue
			}

			resources = append(
				resources,
				state.Resource{
					ID: resourceIDFromARN(
						*resource.Arn,
					),
					ARN:  *resource.Arn,
					Type: resourceType,
					Service: stringValue(
						resource.Service,
					),
					Region: stringValue(
						resource.Region,
					),
				},
			)
		}
	}

	return resources, nil
}

func resourceIDFromARN(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) != 6 {
		return arn
	}

	resource := parts[5]

	switch {
	case strings.HasPrefix(resource, "function:"):
		return strings.TrimPrefix(resource, "function:")

	case strings.HasPrefix(resource, "db:"):
		return strings.TrimPrefix(resource, "db:")

	case strings.HasPrefix(resource, "cluster:"):
		return strings.TrimPrefix(resource, "cluster:")

	case strings.HasPrefix(resource, "table/"):
		return strings.TrimPrefix(resource, "table/")

	case strings.HasPrefix(resource, "bucket:"):
		return strings.TrimPrefix(resource, "bucket:")

	default:
		if index := strings.LastIndex(resource, "/"); index >= 0 {
			return resource[index+1:]
		}

		return resource
	}
}
