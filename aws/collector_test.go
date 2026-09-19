package aws

import (
	"reflect"
	"testing"

	"infraresc/state"
)

func TestNormalizeEdgesConvertsNativeIDsToCanonicalARNs(t *testing.T) {
	resources := []state.Resource{
		{ID: "arn:aws:ec2:ap-south-1:123456789012:instance/i-1", ARN: "arn:aws:ec2:ap-south-1:123456789012:instance/i-1", Type: "AWS::EC2::Instance"},
		{ID: "arn:aws:ec2:ap-south-1:123456789012:subnet/subnet-1", ARN: "arn:aws:ec2:ap-south-1:123456789012:subnet/subnet-1", Type: "AWS::EC2::Subnet"},
		{ID: "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-1", ARN: "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-1", Type: "AWS::EC2::VPC"},
	}
	edges := []state.Edge{
		{From: "i-1", To: "subnet-1", Relation: "IN_SUBNET"},
		{From: "subnet-1", To: "vpc-1", Relation: "BELONGS_TO_VPC"},
		{From: "i-1", To: "subnet-1", Relation: "IN_SUBNET"},
		{From: "missing", To: "vpc-1", Relation: "IGNORED"},
		{From: "i-1", To: "i-1", Relation: "SELF"},
	}
	got := normalizeEdges(resources, edges)
	want := []state.Edge{
		{From: resources[0].ARN, To: resources[1].ARN, Relation: "IN_SUBNET"},
		{From: resources[1].ARN, To: resources[2].ARN, Relation: "BELONGS_TO_VPC"},
	}
	if !reflect.DeepEqual(got, want) { t.Fatalf("normalizeEdges = %#v, want %#v", got, want) }
}

func TestNormalizeEdgesRejectsAmbiguousNativeIDs(t *testing.T) {
	resources := []state.Resource{
		{ARN: "arn:aws:ec2:region-a:111111111111:instance/i-1"},
		{ARN: "arn:aws:ec2:region-b:222222222222:instance/i-1"},
		{ARN: "arn:aws:ec2:region-a:111111111111:vpc/vpc-1"},
	}
	edges := []state.Edge{{From: "i-1", To: "vpc-1", Relation: "IN_VPC"}}
	if got := normalizeEdges(resources, edges); len(got) != 0 { t.Fatalf("expected ambiguous native ID to be dropped, got %#v", got) }
}
