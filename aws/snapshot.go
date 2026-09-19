package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"infraresc/state"
)

type SnapshotCollector struct {
	client *Client
}

func NewSnapshotCollector(client *Client) *SnapshotCollector {
	return &SnapshotCollector{
		client: client,
	}
}

func (s *SnapshotCollector) Collect(
	ctx context.Context,
	accountID string,
	region string,
	resources []state.Resource,
	edges []state.Edge,
) (*state.Snapshot, error) {

	snapshot := &state.Snapshot{
		Version:   "0.2",
		CreatedAt: time.Now().UTC(),

		AccountID: accountID,
		Region:    region,

		Resources: resources,
		Edges:     edges,

		Configs:  make([]state.ResourceConfig, 0),
		Warnings: make([]state.SnapshotWarning, 0),
	}

	addConfigs := func(
		configs []state.ResourceConfig,
		warnings []state.SnapshotWarning,
	) {
		snapshot.Configs = append(snapshot.Configs, configs...)
		snapshot.Warnings = append(snapshot.Warnings, warnings...)
	}

	// ------------------------------------------------------------
	// EC2
	// ------------------------------------------------------------

	configs, warnings := s.collectEC2(ctx, resources)
	addConfigs(configs, warnings)

	// ------------------------------------------------------------
	// S3
	// ------------------------------------------------------------

	configs, warnings = s.collectS3(ctx, resources)
	addConfigs(configs, warnings)

	// ------------------------------------------------------------
	// Lambda
	// ------------------------------------------------------------

	configs, warnings = s.collectLambda(ctx, resources)
	addConfigs(configs, warnings)

	// ------------------------------------------------------------
	// DynamoDB
	// ------------------------------------------------------------

	configs, warnings = s.collectDynamoDB(ctx, resources)
	addConfigs(configs, warnings)

	// ------------------------------------------------------------
	// RDS
	// ------------------------------------------------------------

	configs, warnings = s.collectRDS(ctx, resources)
	addConfigs(configs, warnings)

	return snapshot, nil
}

// configFromValue converts an AWS SDK response into the generic snapshot
// representation.
func configFromValue(
	resource state.Resource,
	value interface{},
) (state.ResourceConfig, error) {

	data, err := json.Marshal(value)
	if err != nil {
		return state.ResourceConfig{}, err
	}

	var properties map[string]interface{}

	if err := json.Unmarshal(data, &properties); err != nil {
		return state.ResourceConfig{}, err
	}

	return state.ResourceConfig{
		ResourceID: resource.ID,
		ARN:        resource.ARN,
		Type:       resource.Type,
		Service:    resource.Service,
		Region:     resource.Region,
		Properties: properties,
	}, nil
}

func snapshotWarning(
	resource state.Resource,
	err error,
) state.SnapshotWarning {
	return state.SnapshotWarning{
		ResourceID: resource.ID,
		Type:       resource.Type,
		Message:    err.Error(),
	}
}

func resourceMap(
	resources []state.Resource,
) map[string]state.Resource {

	result := make(map[string]state.Resource, len(resources))

	for _, resource := range resources {
		result[resource.ID] = resource
	}

	return result
}

func snapshotResourceType(
	resources []state.Resource,
	resourceType string,
) []state.Resource {

	var result []state.Resource

	for _, resource := range resources {
		if resource.Type == resourceType {
			result = append(result, resource)
		}
	}

	return result
}

func unsupportedSnapshotService(
	resource state.Resource,
) state.SnapshotWarning {
	return state.SnapshotWarning{
		ResourceID: resource.ID,
		Type:       resource.Type,
		Message: fmt.Sprintf(
			"snapshot provider does not support resource type %s",
			resource.Type,
		),
	}
}
