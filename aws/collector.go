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

	// Relationship discovery operates using native AWS IDs.
	// Persisted InfraResc state uses canonical ARNs.
	edges = normalizeEdges(
		resources,
		edges,
	)

	return &state.Infrastructure{
		Resources: resources,
		Edges:     edges,
	}, nil
}

// normalizeEdges converts native AWS resource IDs into
// canonical ARN identities.
//
// Example:
//
//	i-123456
//
// becomes:
//
//	arn:aws:ec2:region:account:instance/i-123456
//
// The relationship discovery layer can therefore continue
// using native AWS identifiers while the graph/state layer
// remains ARN-based.
func normalizeEdges(
	resources []state.Resource,
	edges []state.Edge,
) []state.Edge {

	byNativeID := make(
		map[string][]string,
	)

	byARN := make(
		map[string]struct{},
	)

	for _, resource := range resources {

		if resource.ARN == "" {
			continue
		}

		byARN[resource.ARN] = struct{}{}

		nativeID := resourceIDFromARN(
			resource.ARN,
		)

		if nativeID == "" {
			continue
		}

		byNativeID[nativeID] = append(
			byNativeID[nativeID],
			resource.ARN,
		)
	}

	resolve := func(
		id string,
	) string {

		if id == "" {
			return ""
		}

		// Already canonical.
		if _, exists := byARN[id]; exists {
			return id
		}

		candidates := byNativeID[id]

		// Never guess when the ID is ambiguous.
		if len(candidates) != 1 {
			return ""
		}

		return candidates[0]
	}

	result := make(
		[]state.Edge,
		0,
		len(edges),
	)

	seen := make(
		map[string]struct{},
		len(edges),
	)

	for _, edge := range edges {

		from := resolve(edge.From)
		to := resolve(edge.To)

		if from == "" ||
			to == "" ||
			from == to {
			continue
		}

		normalized := state.Edge{
			From:     from,
			To:       to,
			Relation: edge.Relation,
		}

		key :=
			normalized.From +
				"\x00" +
				normalized.To +
				"\x00" +
				normalized.Relation

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}

		result = append(
			result,
			normalized,
		)
	}

	return result
}
