package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"infraresc/state"
)

func (s *SnapshotCollector) collectDynamoDB(
	ctx context.Context,
	resources []state.Resource,
) ([]state.ResourceConfig, []state.SnapshotWarning) {

	if s.client.DynamoDB == nil {
		return nil, []state.SnapshotWarning{
			{
				Type:    "AWS::DynamoDB",
				Message: "DynamoDB client is not initialized",
			},
		}
	}

	var configs []state.ResourceConfig
	var warnings []state.SnapshotWarning

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::DynamoDB::Table",
	) {

		out, err := s.client.DynamoDB.DescribeTable(
			ctx,
			&dynamodb.DescribeTableInput{
				TableName: &resource.ID,
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		if out.Table != nil {

			config, err := configFromValue(
				resource,
				out.Table,
			)

			if err != nil {
				warnings = append(
					warnings,
					snapshotWarning(resource, err),
				)
				continue
			}

			configs = append(configs, config)
		}

		// --------------------------------------------------------
		// Continuous backups
		// --------------------------------------------------------

		backups, err := s.client.DynamoDB.DescribeContinuousBackups(
			ctx,
			&dynamodb.DescribeContinuousBackupsInput{
				TableName: &resource.ID,
			},
		)

		if err == nil {

			if config, err := configFromValue(
				resource,
				backups,
			); err == nil {

				config.Properties["snapshot_component"] =
					"continuous_backups"

				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Point-in-time recovery
		// --------------------------------------------------------

		pitr, err := s.client.DynamoDB.DescribeContinuousBackups(
			ctx,
			&dynamodb.DescribeContinuousBackupsInput{
				TableName: &resource.ID,
			},
		)

		if err == nil {

			if config, err := configFromValue(
				resource,
				pitr,
			); err == nil {

				config.Properties["snapshot_component"] =
					"point_in_time_recovery"

				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Tags
		// --------------------------------------------------------

		if out.Table != nil && out.Table.TableArn != nil {

			tags, err := s.client.DynamoDB.ListTagsOfResource(
				ctx,
				&dynamodb.ListTagsOfResourceInput{
					ResourceArn: out.Table.TableArn,
				},
			)

			if err == nil {

				if config, err := configFromValue(
					resource,
					tags,
				); err == nil {

					config.Properties["snapshot_component"] =
						"tags"

					configs = append(configs, config)
				}
			}
		}
	}

	return configs, warnings
}
