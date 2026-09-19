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

			// Resource Explorer gives us the canonical AWS ARN.
			// The ARN is the identity used throughout InfraResc.
			//
			// resourceIDFromARN() must only be used when an
			// AWS service API requires the native identifier.
			arn := *resource.Arn

			resources = append(
				resources,
				state.Resource{
					ID:   arn,
					ARN:  arn,
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

// resourceIDFromARN converts a canonical AWS ARN into the
// native identifier expected by service-specific AWS APIs.
//
// IMPORTANT:
// This function must NOT be used to populate state.Resource.ID.
// InfraResc uses the full ARN as the canonical resource identity.
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
