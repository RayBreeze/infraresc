package aws

import (
	"reflect"
	"testing"

	"infraresc/state"
)

func TestIDsByTypeFiltersAndDeduplicatesResources(t *testing.T) {
	resources := []state.Resource{
		{ID: "i-1", Type: "AWS::EC2::Instance"},
		{ID: "i-1", Type: "AWS::EC2::Instance"},
		{ID: "i-2", Type: "AWS::EC2::Instance"},
		{ID: "", Type: "AWS::EC2::Instance"},
		{ID: "subnet-1", Type: "AWS::EC2::Subnet"},
		{ID: "ignored", Type: ""},
	}

	got := idsByType(resources)

	want := map[string][]string{
		"AWS::EC2::Instance": {"i-1", "i-2"},
		"AWS::EC2::Subnet":   {"subnet-1"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("idsByType() = %#v, want %#v", got, want)
	}
}

func TestChunk(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e"}

	got := chunk(values, 2)
	want := [][]string{
		{"a", "b"},
		{"c", "d"},
		{"e"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chunk() = %#v, want %#v", got, want)
	}

	if got := chunk(nil, 2); got != nil {
		t.Fatalf("chunk(nil, 2) = %#v, want nil", got)
	}

	got = chunk(values, 0)
	if len(got) != 1 || !reflect.DeepEqual(got[0], values) {
		t.Fatalf("chunk() with non-positive size should use default size: %#v", got)
	}
}

func TestDeduplicateEdges(t *testing.T) {
	edges := []state.Edge{
		{From: "instance", To: "subnet", Relation: "IN_SUBNET"},
		{From: "instance", To: "subnet", Relation: "IN_SUBNET"},
		{From: "instance", To: "vpc", Relation: "IN_VPC"},
		{From: "instance", To: "subnet", Relation: "USES_SECURITY_GROUP"},
	}

	got := deduplicateEdges(edges)

	want := []state.Edge{
		{From: "instance", To: "subnet", Relation: "IN_SUBNET"},
		{From: "instance", To: "vpc", Relation: "IN_VPC"},
		{From: "instance", To: "subnet", Relation: "USES_SECURITY_GROUP"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deduplicateEdges() = %#v, want %#v", got, want)
	}
}

func TestStr(t *testing.T) {
	value := "value"

	if got := str(&value); got != "value" {
		t.Fatalf("str(&value) = %q, want %q", got, value)
	}

	if got := str(nil); got != "" {
		t.Fatalf("str(nil) = %q, want empty string", got)
	}
}

func TestServiceResourceIDFromARN(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{
			name: "dynamodb stream",
			arn:  "arn:aws:dynamodb:ap-south-1:123456789012:table/orders/stream/2026-09-19T12:00:00.000",
			want: "orders",
		},
		{
			name: "sqs queue",
			arn:  "arn:aws:sqs:ap-south-1:123456789012:orders",
			want: "orders",
		},
		{
			name: "lambda function",
			arn:  "arn:aws:lambda:ap-south-1:123456789012:function:processor",
			want: "processor",
		},
		{
			name: "empty ARN",
			arn:  "",
			want: "",
		},
		{
			name: "malformed ARN",
			arn:  "not-an-arn",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceResourceIDFromARN(tt.arn); got != tt.want {
				t.Fatalf("serviceResourceIDFromARN(%q) = %q, want %q", tt.arn, got, tt.want)
			}
		})
	}
}
