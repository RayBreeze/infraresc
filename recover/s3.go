package recover

import (
	"context"
	"fmt"

	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"infraresc/state"
)

func bucketNameFromARN(arn string) string {
	const prefix = "arn:aws:s3:::"

	if !strings.HasPrefix(arn, prefix) {
		return ""
	}

	return strings.TrimPrefix(arn, prefix)
}

func (e *Engine) recoverS3(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	bucket := bucketNameFromARN(item.ResourceID)

	if bucket == "" {
		return fmt.Errorf(
			"cannot determine S3 bucket name from %s",
			item.ResourceID,
		)
	}

	_, err := e.AWS.S3.CreateBucket(
		ctx,
		&s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		},
	)

	if err != nil {
		return fmt.Errorf(
			"create S3 bucket %s: %w",
			bucket,
			err,
		)
	}

	return nil
}
