package integration_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"infraresc/graph"
	"infraresc/state"
)

func TestSnapshotSerializationFeedsGraphConstruction(t *testing.T) {
	createdAt := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	original := state.Snapshot{
		Version:   "0.2",
		CreatedAt: createdAt,
		AccountID: "123456789012",
		Region:    "ap-south-1",
		Resources: []state.Resource{
			{ID: "vpc-1", ARN: "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-1", Type: "AWS::EC2::VPC", Service: "ec2", Region: "ap-south-1"},
			{ID: "subnet-1", ARN: "arn:aws:ec2:ap-south-1:123456789012:subnet/subnet-1", Type: "AWS::EC2::Subnet", Service: "ec2", Region: "ap-south-1"},
		},
		Edges: []state.Edge{
			{From: "subnet-1", To: "vpc-1", Relation: "belongs-to"},
		},
		Configs: []state.ResourceConfig{
			{ResourceID: "vpc-1", ARN: "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-1", Type: "AWS::EC2::VPC", Service: "ec2", Region: "ap-south-1", Properties: map[string]interface{}{"cidr": "10.0.0.0/16"}},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var restored state.Snapshot
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("snapshot changed across serialization: %#v != %#v", original, restored)
	}

	g := graph.Build(restored.Resources)
	if len(g.Nodes) != len(restored.Resources) {
		t.Fatalf("graph has %d nodes, want %d", len(g.Nodes), len(restored.Resources))
	}

	for _, resource := range restored.Resources {
		node, ok := g.Nodes[resource.ID]
		if !ok {
			t.Fatalf("resource %q did not reach graph construction", resource.ID)
		}
		if node.ResourceType != resource.Type {
			t.Fatalf("resource %q changed type during graph construction: got %q, want %q", resource.ID, node.ResourceType, resource.Type)
		}
	}

	order, err := g.ResolveOrder()
	if err != nil {
		t.Fatalf("resolve graph order: %v", err)
	}
	if len(order) != len(restored.Resources) {
		t.Fatalf("resolved %d resources, want %d", len(order), len(restored.Resources))
	}
}
