package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"infraresc/state"
)

func (s *SnapshotCollector) collectRDS(
	ctx context.Context,
	resources []state.Resource,
) ([]state.ResourceConfig, []state.SnapshotWarning) {

	if s.client.RDS == nil {
		return nil, []state.SnapshotWarning{
			{
				Type:    "AWS::RDS",
				Message: "RDS client is not initialized",
			},
		}
	}

	var configs []state.ResourceConfig
	var warnings []state.SnapshotWarning

	// ------------------------------------------------------------
	// DB Instances
	// ------------------------------------------------------------

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::RDS::DBInstance",
	) {

		apiID := resourceIDFromARN(resource.ARN)

		out, err := s.client.RDS.DescribeDBInstances(
			ctx,
			&rds.DescribeDBInstancesInput{
				DBInstanceIdentifier: &apiID,
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		for _, instance := range out.DBInstances {

			config, err := configFromValue(
				resource,
				instance,
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
	}

	// ------------------------------------------------------------
	// DB Clusters
	// ------------------------------------------------------------

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::RDS::DBCluster",
	) {

		apiID := resourceIDFromARN(resource.ARN)

		out, err := s.client.RDS.DescribeDBClusters(
			ctx,
			&rds.DescribeDBClustersInput{
				DBClusterIdentifier: &apiID,
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		for _, cluster := range out.DBClusters {

			config, err := configFromValue(
				resource,
				cluster,
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
	}

	// ------------------------------------------------------------
	// DB Subnet Groups
	// ------------------------------------------------------------

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::RDS::DBSubnetGroup",
	) {

		apiID := resourceIDFromARN(resource.ARN)

		out, err := s.client.RDS.DescribeDBSubnetGroups(
			ctx,
			&rds.DescribeDBSubnetGroupsInput{
				DBSubnetGroupName: &apiID,
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		for _, group := range out.DBSubnetGroups {

			config, err := configFromValue(
				resource,
				group,
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
	}

	return configs, warnings
}
