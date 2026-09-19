package graph

import (
	"fmt"
	"sort"

	"infraresc/state"
)

func Build(resources []state.Resource) *Graph {
	g := New()

	for _, resource := range resources {
		g.AddNode(&Node{
			ID:           resource.ID,
			ResourceType: resource.Type,
		})
	}

	return g
}

// BuildWithEdges builds a dependency graph from discovered resources
// and canonical relationship edges.
//
// If A -> B, A depends on B.
// Therefore B must be recovered before A.
func BuildWithEdges(
	resources []state.Resource,
	edges []state.Edge,
) *Graph {

	g := Build(resources)

	for _, edge := range edges {

		from, fromExists := g.Nodes[edge.From]
		_, toExists := g.Nodes[edge.To]

		if !fromExists || !toExists {
			continue
		}

		if edge.From == edge.To {
			continue
		}

		from.Dependencies = append(
			from.Dependencies,
			edge.To,
		)
	}

	for _, node := range g.Nodes {
		node.Dependencies = uniqueSorted(
			node.Dependencies,
		)
	}

	return g
}

// ResolveOrder performs a topological traversal.
//
// Dependencies are emitted before the resources that depend on them.
func (g *Graph) ResolveOrder() ([]string, error) {

	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	order := make(
		[]string,
		0,
		len(g.Nodes),
	)

	// Map iteration is deliberately randomized in Go.
	// Sort node IDs so snapshot generation is deterministic.
	ids := make(
		[]string,
		0,
		len(g.Nodes),
	)

	for id := range g.Nodes {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	var visit func(string) error

	visit = func(id string) error {

		if visiting[id] {
			return fmt.Errorf(
				"dependency cycle detected involving %s",
				id,
			)
		}

		if visited[id] {
			return nil
		}

		node, exists := g.Nodes[id]

		if !exists {
			return fmt.Errorf(
				"dependency %s not found",
				id,
			)
		}

		visiting[id] = true

		for _, dependency := range node.Dependencies {

			if err := visit(dependency); err != nil {
				return err
			}
		}

		visiting[id] = false
		visited[id] = true

		order = append(
			order,
			id,
		)

		return nil
	}

	for _, id := range ids {

		if err := visit(id); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// Snapshot converts the in-memory graph into the portable state format.
func (g *Graph) Snapshot() state.DependencyGraph {

	ids := make(
		[]string,
		0,
		len(g.Nodes),
	)

	for id := range g.Nodes {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	nodes := make(
		[]state.GraphNode,
		0,
		len(ids),
	)

	for _, id := range ids {

		node := g.Nodes[id]

		nodes = append(
			nodes,
			state.GraphNode{
				ID:           node.ID,
				ResourceType: node.ResourceType,
				Dependencies: append(
					[]string(nil),
					node.Dependencies...,
				),
			},
		)
	}

	order, err := g.ResolveOrder()

	if err != nil {
		order = nil
	}

	return state.DependencyGraph{
		Nodes: nodes,
		Order: order,
	}
}

func uniqueSorted(
	values []string,
) []string {

	seen := make(
		map[string]struct{},
		len(values),
	)

	result := make(
		[]string,
		0,
		len(values),
	)

	for _, value := range values {

		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}

		result = append(
			result,
			value,
		)
	}

	sort.Strings(result)

	return result
}
