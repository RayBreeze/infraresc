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
	if err != nil { t.Fatalf("marshal snapshot: %v", err) }

	var restored state.Snapshot
	if err := json.Unmarshal(data, &restored); err != nil { t.Fatalf("unmarshal snapshot: %v", err) }
	if !reflect.DeepEqual(original, restored) { t.Fatalf("snapshot changed across serialization: %#v != %#v", original, restored) }

	g := graph.BuildWithEdges(restored.Resources, restored.Edges)
	if len(g.Nodes) != len(restored.Resources) { t.Fatalf("graph has %d nodes, want %d", len(g.Nodes), len(restored.Resources)) }

	if got := g.Nodes["subnet-1"].Dependencies; !reflect.DeepEqual(got, []string{"vpc-1"}) {
		t.Fatalf("subnet dependencies = %v, want [vpc-1]", got)
	}

	order, err := g.ResolveOrder()
	if err != nil { t.Fatalf("resolve graph order: %v", err) }
	wantOrder := []string{"vpc-1", "subnet-1"}
	if !reflect.DeepEqual(order, wantOrder) { t.Fatalf("resolved order = %v, want %v", order, wantOrder) }

	graphSnapshot := g.Snapshot()
	if !reflect.DeepEqual(graphSnapshot.Order, wantOrder) { t.Fatalf("graph snapshot order = %v, want %v", graphSnapshot.Order, wantOrder) }
	if !reflect.DeepEqual(graphSnapshot.Nodes[1].Dependencies, []string{"vpc-1"}) { t.Fatalf("graph snapshot dependencies = %v, want [vpc-1]", graphSnapshot.Nodes[1].Dependencies) }
}
