package graph

import (
	"fmt"
	"infraresc/state"
)

func Build(resources []state.Resource) *Graph {
	g := New()

	for _, resource := range resources {
		node := &Node{
			ID:           resource.ID,
			ResourceType: resource.Type,
			Dependencies: resource.Dependencies,
		}

		g.AddNode(node)
	}

	return g
}

func (g *Graph) ResolveOrder() ([]string, error) {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	order := make([]string, 0, len(g.Nodes))

	var visit func(string) error

	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("dependency cycle detected involving %s", id)
		}

		if visited[id] {
			return nil
		}

		node, exists := g.Nodes[id]
		if !exists {
			return fmt.Errorf("dependency %s not found", id)
		}

		visiting[id] = true

		for _, dependency := range node.Dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}

		visiting[id] = false
		visited[id] = true

		order = append(order, id)

		return nil
	}

	for id := range g.Nodes {
		if err := visit(id); err != nil {
			return nil, err
		}
	}

	return order, nil
}
