package graph

import (
	"reflect"
	"testing"

	"infraresc/state"
)

func TestBuildCreatesNodesFromResources(t *testing.T) {
	resources := []state.Resource{
		{ID: "vpc-123", Type: "AWS::EC2::VPC"},
		{ID: "subnet-456", Type: "AWS::EC2::Subnet"},
	}
	g := Build(resources)
	if len(g.Nodes) != len(resources) { t.Fatalf("expected %d nodes, got %d", len(resources), len(g.Nodes)) }
	for _, resource := range resources {
		node, ok := g.Nodes[resource.ID]
		if !ok { t.Fatalf("expected node %q to exist", resource.ID) }
		if node.ID != resource.ID { t.Errorf("node ID = %q, want %q", node.ID, resource.ID) }
		if node.ResourceType != resource.Type { t.Errorf("node type = %q, want %q", node.ResourceType, resource.Type) }
	}
}

func TestBuildWithEdgesCreatesDependencies(t *testing.T) {
	resources := []state.Resource{
		{ID: "vpc", Type: "AWS::EC2::VPC"},
		{ID: "subnet", Type: "AWS::EC2::Subnet"},
		{ID: "instance", Type: "AWS::EC2::Instance"},
	}
	edges := []state.Edge{
		{From: "subnet", To: "vpc", Relation: "BELONGS_TO_VPC"},
		{From: "instance", To: "subnet", Relation: "IN_SUBNET"},
		{From: "instance", To: "subnet", Relation: "duplicate"},
		{From: "missing", To: "vpc", Relation: "ignored"},
		{From: "vpc", To: "vpc", Relation: "self"},
	}
	g := BuildWithEdges(resources, edges)
	if got := g.Nodes["subnet"].Dependencies; !reflect.DeepEqual(got, []string{"vpc"}) { t.Fatalf("subnet dependencies = %v, want [vpc]", got) }
	if got := g.Nodes["instance"].Dependencies; !reflect.DeepEqual(got, []string{"subnet"}) { t.Fatalf("instance dependencies = %v, want [subnet]", got) }
	if got := g.Nodes["vpc"].Dependencies; len(got) != 0 { t.Fatalf("vpc dependencies = %v, want none", got) }
}

func TestResolveOrderRespectsDependencies(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "vpc"})
	g.AddNode(&Node{ID: "subnet", Dependencies: []string{"vpc"}})
	g.AddNode(&Node{ID: "instance", Dependencies: []string{"subnet"}})
	order, err := g.ResolveOrder()
	if err != nil { t.Fatalf("ResolveOrder returned error: %v", err) }
	positions := make(map[string]int, len(order))
	for i, id := range order { positions[id] = i }
	if positions["vpc"] >= positions["subnet"] { t.Fatalf("dependency vpc must be resolved before subnet: %v", order) }
	if positions["subnet"] >= positions["instance"] { t.Fatalf("dependency subnet must be resolved before instance: %v", order) }
}

func TestResolveOrderIsDeterministicForIndependentNodes(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})
	g.AddNode(&Node{ID: "a"})
	want := []string{"a", "b", "c"}
	for i := 0; i < 5; i++ {
		order, err := g.ResolveOrder()
		if err != nil { t.Fatalf("ResolveOrder returned error: %v", err) }
		if !reflect.DeepEqual(order, want) { t.Fatalf("ResolveOrder = %v, want %v", order, want) }
	}
}

func TestGraphSnapshotIsDeterministicAndPreservesDependencies(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "b", ResourceType: "B", Dependencies: []string{"a"}})
	g.AddNode(&Node{ID: "a", ResourceType: "A"})
	got := g.Snapshot()
	want := state.DependencyGraph{
		Nodes: []state.GraphNode{
			{ID: "a", ResourceType: "A"},
			{ID: "b", ResourceType: "B", Dependencies: []string{"a"}},
		},
		Order: []string{"a", "b"},
	}
	if !reflect.DeepEqual(got, want) { t.Fatalf("graph snapshot = %#v, want %#v", got, want) }
}

func TestResolveOrderRejectsMissingDependency(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "subnet", Dependencies: []string{"missing-vpc"}})
	_, err := g.ResolveOrder()
	if err == nil { t.Fatal("expected missing dependency to return an error") }
	if err.Error() != "dependency missing-vpc not found" { t.Fatalf("unexpected error: %v", err) }
}

func TestResolveOrderRejectsDependencyCycle(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Dependencies: []string{"b"}})
	g.AddNode(&Node{ID: "b", Dependencies: []string{"c"}})
	g.AddNode(&Node{ID: "c", Dependencies: []string{"a"}})
	_, err := g.ResolveOrder()
	if err == nil { t.Fatal("expected dependency cycle to return an error") }
	if err.Error() != "dependency cycle detected involving a" && err.Error() != "dependency cycle detected involving b" && err.Error() != "dependency cycle detected involving c" { t.Fatalf("unexpected cycle error: %v", err) }
}

func TestResolveOrderIncludesIndependentNodes(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})
	order, err := g.ResolveOrder()
	if err != nil { t.Fatalf("ResolveOrder returned error: %v", err) }
	if len(order) != 3 { t.Fatalf("expected all 3 nodes in order, got %d: %v", len(order), order) }
	seen := make(map[string]bool, len(order))
	for _, id := range order { if seen[id] { t.Fatalf("node %q appears more than once in order: %v", id, order) }; seen[id] = true }
	for _, id := range []string{"a", "b", "c"} { if !seen[id] { t.Fatalf("resolved order is missing node %q: %v", id, order) } }
}
