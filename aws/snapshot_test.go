package aws

import (
	"reflect"
	"testing"
	"time"

	"infraresc/state"
)

func TestConfigFromValuePreservesResourceIdentityAndProperties(t *testing.T) {
	resource := state.Resource{
		ID:      "i-123",
		ARN:     "arn:aws:ec2:ap-south-1:123456789012:instance/i-123",
		Type:    "AWS::EC2::Instance",
		Service: "ec2",
		Region:  "ap-south-1",
	}

	value := struct {
		InstanceType string
		Running      bool
	}{
		InstanceType: "t3.micro",
		Running:      true,
	}

	config, err := configFromValue(resource, value)
	if err != nil {
		t.Fatalf("configFromValue returned error: %v", err)
	}

	if config.ResourceID != resource.ID ||
		config.ARN != resource.ARN ||
		config.Type != resource.Type ||
		config.Service != resource.Service ||
		config.Region != resource.Region {
		t.Fatalf("config did not preserve resource identity: %#v", config)
	}

	if got := config.Properties["InstanceType"]; got != "t3.micro" {
		t.Fatalf("InstanceType = %#v, want %q", got, "t3.micro")
	}
	if got := config.Properties["Running"]; got != true {
		t.Fatalf("Running = %#v, want true", got)
	}
}

func TestSnapshotResourceTypeFiltersOnlyRequestedType(t *testing.T) {
	resources := []state.Resource{
		{ID: "i-1", Type: "AWS::EC2::Instance"},
		{ID: "subnet-1", Type: "AWS::EC2::Subnet"},
		{ID: "i-2", Type: "AWS::EC2::Instance"},
	}

	got := snapshotResourceType(resources, "AWS::EC2::Instance")
	want := []state.Resource{resources[0], resources[2]}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshotResourceType() = %#v, want %#v", got, want)
	}
}

func TestResourceMapIndexesResourcesByID(t *testing.T) {
	resources := []state.Resource{
		{ID: "vpc-1", Type: "AWS::EC2::VPC"},
		{ID: "subnet-1", Type: "AWS::EC2::Subnet"},
	}

	got := resourceMap(resources)

	if len(got) != 2 {
		t.Fatalf("resourceMap() returned %d entries, want 2", len(got))
	}
	if got["vpc-1"] != resources[0] {
		t.Fatalf("resourceMap() lost vpc-1 resource")
	}
	if got["subnet-1"] != resources[1] {
		t.Fatalf("resourceMap() lost subnet-1 resource")
	}
}

func TestSnapshotWarningContainsResourceContext(t *testing.T) {
	resource := state.Resource{
		ID:   "table-1",
		Type: "AWS::DynamoDB::Table",
	}

	warning := snapshotWarning(resource, assertErr("describe failed"))

	if warning.ResourceID != resource.ID {
		t.Fatalf("warning ResourceID = %q, want %q", warning.ResourceID, resource.ID)
	}
	if warning.Type != resource.Type {
		t.Fatalf("warning Type = %q, want %q", warning.Type, resource.Type)
	}
	if warning.Message != "describe failed" {
		t.Fatalf("warning Message = %q, want %q", warning.Message, "describe failed")
	}
}

func TestSnapshotCollectorCollectInitializesSnapshotMetadata(t *testing.T) {
	collector := NewSnapshotCollector(&Client{})
	before := time.Now().UTC()

	snapshot, err := collector.Collect(
		testingContext{},
		"123456789012",
		"ap-south-1",
		[]state.Resource{},
		[]state.Edge{},
	)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	after := time.Now().UTC()

	if snapshot.Version != "0.2" {
		t.Fatalf("snapshot Version = %q, want %q", snapshot.Version, "0.2")
	}
	if snapshot.AccountID != "123456789012" {
		t.Fatalf("snapshot AccountID = %q", snapshot.AccountID)
	}
	if snapshot.Region != "ap-south-1" {
		t.Fatalf("snapshot Region = %q", snapshot.Region)
	}
	if snapshot.CreatedAt.Before(before) || snapshot.CreatedAt.After(after) {
		t.Fatalf("snapshot CreatedAt %v is outside collection interval", snapshot.CreatedAt)
	}
	if snapshot.Resources == nil || snapshot.Edges == nil ||
		snapshot.Configs == nil || snapshot.Warnings == nil {
		t.Fatalf("snapshot collection should initialize all slice fields")
	}

	if len(snapshot.Warnings) != 5 {
		t.Fatalf("with nil service clients, expected 5 service warnings, got %d", len(snapshot.Warnings))
	}
}

func assertErr(message string) error {
	return testError(message)
}

type testError string

func (e testError) Error() string { return string(e) }

type testingContext struct{}

func (testingContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (testingContext) Done() <-chan struct{}       { return nil }
func (testingContext) Err() error                   { return nil }
func (testingContext) Value(any) any               { return nil }
