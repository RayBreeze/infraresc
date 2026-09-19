package recover

import (
	"fmt"

	"infraresc/graph"
	"infraresc/state"
)

func RecoveryOrder(
	snapshot *state.Snapshot,
	items map[string]state.RecoveryItem,
) ([]string, error) {

	g := graph.BuildWithEdges(
		snapshot.Resources,
		snapshot.Edges,
	)

	order, err := g.ResolveOrder()

	if err != nil {
		return nil, fmt.Errorf(
			"resolve recovery dependencies: %w",
			err,
		)
	}

	result := make(
		[]string,
		0,
		len(items),
	)

	for _, id := range order {

		if _, exists := items[id]; exists {
			result = append(result, id)
		}
	}

	return result, nil
}
