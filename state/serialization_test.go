package state

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSerializeDeserializeSnapshotRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 9, 19, 12, 34, 56, 0, time.UTC)
	original := Snapshot{
		Version:   "1",
		CreatedAt: createdAt,
		AccountID: "123456789012",
		Region:    "ap-south-1",
		Resources: []Resource{
			{
				ID:      "vpc-123",
				ARN:     "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-123",
				Type:    "AWS::EC2::VPC",
				Service: "ec2",
				Region:  "ap-south-1",
			},
		},
		Edges: []Edge{
			{From: "subnet-456", To: "vpc-123", Relation: "belongs_to"},
		},
		Graph: DependencyGraph{
			Nodes: []GraphNode{
				{ID: "vpc-123", ResourceType: "AWS::EC2::VPC"},
				{
					ID:           "subnet-456",
					ResourceType: "AWS::EC2::Subnet",
					Dependencies: []string{"vpc-123"},
				},
			},
			Order: []string{"vpc-123", "subnet-456"},
		},
		Configs: []ResourceConfig{
			{
				ResourceID: "vpc-123",
				ARN:        "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-123",
				Type:       "AWS::EC2::VPC",
				Service:    "ec2",
				Region:     "ap-south-1",
				Properties: map[string]interface{}{
					"cidr_block": "10.0.0.0/16",
					"enable_dns": true,
					"tags": map[string]interface{}{
						"Name": "test-vpc",
					},
				},
			},
		},
		Warnings: []SnapshotWarning{
			{
				ResourceID: "vpc-123",
				Type:       "partial",
				Message:    "some configuration fields were unavailable",
			},
		},
	}

	data, err := SerializeSnapshot(original)
	if err != nil {
		t.Fatalf("SerializeSnapshot returned error: %v", err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("serialized snapshot is not valid JSON: %v", err)
	}
	decoded, err = DeserializeSnapshot(data)
	if err != nil {
		t.Fatalf("DeserializeSnapshot returned error: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip changed snapshot
original: %#v
decoded: %#v", original, decoded)
	}
}

func TestSerializeSnapshotOmitsEmptyWarnings(t *testing.T) {
	snapshot := Snapshot{
		Version:   "1",
		CreatedAt: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		AccountID: "123456789012",
		Region:    "ap-south-1",
	}
	data, err := SerializeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("SerializeSnapshot returned error: %v", err)
	}
	if strings.Contains(string(data), "warnings") {
		t.Fatalf("expected empty warnings field to be omitted, got: %s", data)
	}
}

func TestDeserializeSnapshotRejectsInvalidJSON(t *testing.T) {
	_, err := DeserializeSnapshot([]byte(`{"version":`))
	if err == nil {
		t.Fatal("expected invalid JSON to return an error")
	}
}
