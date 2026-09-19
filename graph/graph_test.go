package graph

import (
	"testing"

	"infraresc/state"
)

func TestBuildCreatesNodesFromResources(t *testing.T) {
	resources := []state.Resource{
		{
			ID:   "vpc-123",
			Type: "AWS::EC2::VPC",
		},
		{
			ID:   "subnet-456",
			Type: "AWS::EC2::Subnet",
		},
	}

	g := Build(resources)

	if len(g.Nodes) != len(resources) {
		t.Fatalf("expected %d nodes, got %d", len(resources), len(g.Nodes))
	}

	for _, resource := range resources {
		node, ok := g.Nodes[resource.ID]
		if !ok {
			t.Fatalf("expected node %q to exist", resource.ID)
		}
		if node.ID != resource.ID {
			t.Errorf("node ID = %q, want %q", node.ID, resource.ID)
		}
		if node.ResourceType != resource.Type {
			t.Errorf("node type = %q, want %q", node.ResourceType, resource.Type)
		}
	}
}

func TestResolveOrderRespectsDependencies(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "vpc"})
	g.AddNode(&Node{ID: "subnet", Dependencies: []string{"vpc"}})
	g.AddNode(&Node{ID: "instance", Dependencies: []string{"subnet"}})

	order, err := g.ResolveOrder()
	if err != nil {
		t.Fatalf("ResolveOrder returned error: %v", err)
	}

	positions := make(map[string]int, len(order))
	for i, id := range order {
		positions[id] = i
	}

	for _, id := range []string{"vpc", "subnet", "instance"} {
		if _, ok := positions[id]; !ok {
			t.Fatalf("resolved order is missing node %q: %v", id, order)
		}
	}

	if positions["vpc"] >= positions["subnet"] {
		t.Fatalf("dependency vpc must be resolved before subnet: %v", order)
	}
	if positions["subnet"] >= positions["instance"] {
		t.Fatalf("dependency subnet must be resolved before instance: %v", order)
	}
}

func TestResolveOrderRejectsMissingDependency(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "subnet", Dependencies: []string{"missing-vpc"}})

	_, err := g.ResolveOrder()
	if err == nil {
		t.Fatal("expected missing dependency to return an error")
	}
	if err.Error() != "dependency missing-vpc not found" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveOrderRejectsDependencyCycle(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Dependencies: []string{"b"}})
	g.AddNode(&Node{ID: "b", Dependencies: []string{"c"}})
	g.AddNode(&Node{ID: "c", Dependencies: []string{"a"}})

	_, err := g.ResolveOrder()
	if err == nil {
		t.Fatal("expected dependency cycle to return an error")
	}
	if err.Error() != "dependency cycle detected involving a" &&
		err.Error() != "dependency cycle detected involving b" &&
		err.Error() != "dependency cycle detected involving c" {
		t.Fatalf("unexpected cycle error: %v", err)
	}
}

func TestResolveOrderIncludesIndependentNodes(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})

	order, err := g.ResolveOrder()
	if err != nil {
		t.Fatalf("ResolveOrder returned error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected all 3 nodes in order, got %d: %v", len(order), order)
	}

	seen := make(map[string]bool, len(order))
	for _, id := range order {
		if seen[id] {
			t.Fatalf("node %q appears more than once in order: %v", id, order)
		}
		seen[id] = true
	}

	for _, id := range []string{"a", "b", "c"} {
		if !seen[id] {
			t.Fatalf("resolved order is missing node %q: %v", id, order)
		}
	}
}
