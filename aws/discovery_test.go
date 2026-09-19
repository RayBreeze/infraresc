package aws

import "testing"

func TestResourceIDFromARN(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{
			name: "lambda function",
			arn:  "arn:aws:lambda:ap-south-1:123456789012:function:my-function",
			want: "my-function",
		},
		{
			name: "rds instance",
			arn:  "arn:aws:rds:ap-south-1:123456789012:db:my-db",
			want: "my-db",
		},
		{
			name: "rds cluster",
			arn:  "arn:aws:rds:ap-south-1:123456789012:cluster:my-cluster",
			want: "my-cluster",
		},
		{
			name: "dynamodb table",
			arn:  "arn:aws:dynamodb:ap-south-1:123456789012:table/my-table",
			want: "my-table",
		},
		{
			name: "s3 bucket",
			arn:  "arn:aws:s3:::bucket:my-bucket",
			want: "my-bucket",
		},
		{
			name: "slash based resource",
			arn:  "arn:aws:ec2:ap-south-1:123456789012:subnet/subnet-123",
			want: "subnet-123",
		},
		{
			name: "plain resource",
			arn:  "arn:aws:sns:ap-south-1:123456789012:topic-name",
			want: "topic-name",
		},
		{
			name: "malformed ARN",
			arn:  "not-an-arn",
			want: "not-an-arn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceIDFromARN(tt.arn); got != tt.want {
				t.Fatalf("resourceIDFromARN(%q) = %q, want %q", tt.arn, got, tt.want)
			}
		})
	}
}

func TestStringValue(t *testing.T) {
	value := "ap-south-1"

	if got := stringValue(&value); got != value {
		t.Fatalf("stringValue(&value) = %q, want %q", got, value)
	}

	if got := stringValue(nil); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty string", got)
	}
}
