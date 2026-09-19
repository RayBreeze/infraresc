package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"infraresc/state"
)

func (s *SnapshotCollector) collectS3(
	ctx context.Context,
	resources []state.Resource,
) ([]state.ResourceConfig, []state.SnapshotWarning) {

	if s.client.S3 == nil {
		return nil, []state.SnapshotWarning{
			{
				Type:    "AWS::S3",
				Message: "S3 client is not initialized",
			},
		}
	}

	var configs []state.ResourceConfig
	var warnings []state.SnapshotWarning

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::S3::Bucket",
	) {

		bucket := resource.ID

		// --------------------------------------------------------
		// Basic bucket information
		// --------------------------------------------------------

		location, err := s.client.S3.GetBucketLocation(
			ctx,
			&s3.GetBucketLocationInput{
				Bucket: &bucket,
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
		} else {
			if config, err := configFromValue(
				resource,
				location,
			); err == nil {
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Versioning
		// --------------------------------------------------------

		versioning, err := s.client.S3.GetBucketVersioning(
			ctx,
			&s3.GetBucketVersioningInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				versioning,
			); err == nil {
				config.Properties["snapshot_component"] = "versioning"
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Encryption
		// --------------------------------------------------------

		encryption, err := s.client.S3.GetBucketEncryption(
			ctx,
			&s3.GetBucketEncryptionInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				encryption,
			); err == nil {
				config.Properties["snapshot_component"] = "encryption"
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Lifecycle
		// --------------------------------------------------------

		lifecycle, err := s.client.S3.GetBucketLifecycleConfiguration(
			ctx,
			&s3.GetBucketLifecycleConfigurationInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				lifecycle,
			); err == nil {
				config.Properties["snapshot_component"] = "lifecycle"
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Public access block
		// --------------------------------------------------------

		publicAccess, err := s.client.S3.GetPublicAccessBlock(
			ctx,
			&s3.GetPublicAccessBlockInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				publicAccess,
			); err == nil {
				config.Properties["snapshot_component"] = "public_access_block"
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Bucket policy
		// --------------------------------------------------------

		policy, err := s.client.S3.GetBucketPolicy(
			ctx,
			&s3.GetBucketPolicyInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				policy,
			); err == nil {
				config.Properties["snapshot_component"] = "policy"
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Notification configuration
		// --------------------------------------------------------

		notification, err := s.client.S3.GetBucketNotificationConfiguration(
			ctx,
			&s3.GetBucketNotificationConfigurationInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				notification,
			); err == nil {
				config.Properties["snapshot_component"] = "notifications"
				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Tags
		// --------------------------------------------------------

		tags, err := s.client.S3.GetBucketTagging(
			ctx,
			&s3.GetBucketTaggingInput{
				Bucket: &bucket,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				tags,
			); err == nil {
				config.Properties["snapshot_component"] = "tags"
				configs = append(configs, config)
			}
		}
	}

	return configs, warnings
}
