package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"infraresc/state"
)

// discoverServiceRelationships discovers relationships outside the EC2/VPC
// family. It intentionally operates only on resources already discovered by
// Resource Explorer.
func (r *RelationshipDiscovery) discoverServiceRelationships(
	ctx context.Context,
	ids map[string][]string,
) ([]state.Edge, error) {
	var edges []state.Edge

	add := func(from, to, relation string) {
		if from == "" || to == "" || from == to {
			return
		}

		edges = append(edges, state.Edge{
			From:     from,
			To:       to,
			Relation: relation,
		})
	}

	// ============================================================
	// S3
	// ============================================================

	if r.S3 != nil {
		// S3 Bucket -> Lambda / SQS / SNS notifications.
		for _, bucket := range ids["AWS::S3::Bucket"] {
			out, err := r.S3.GetBucketNotificationConfiguration(
				ctx,
				&s3.GetBucketNotificationConfigurationInput{
					Bucket: &bucket,
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"reading S3 bucket notification configuration for %s: %w",
					bucket,
					err,
				)
			}

			for _, config := range out.LambdaFunctionConfigurations {
				add(
					bucket,
					resourceIDFromARN(str(config.LambdaFunctionArn)),
					"TRIGGERS_LAMBDA",
				)
			}

			for _, config := range out.QueueConfigurations {
				add(
					bucket,
					resourceIDFromARN(str(config.QueueArn)),
					"SENDS_TO_SQS",
				)
			}

			for _, config := range out.TopicConfigurations {
				add(
					bucket,
					resourceIDFromARN(str(config.TopicArn)),
					"PUBLISHES_TO_SNS",
				)
			}
		}

		// S3 Bucket -> KMS key.
		for _, bucket := range ids["AWS::S3::Bucket"] {
			out, err := r.S3.GetBucketEncryption(
				ctx,
				&s3.GetBucketEncryptionInput{
					Bucket: &bucket,
				},
			)
			if err != nil {
				// Not every bucket has explicit customer-managed KMS
				// encryption. AWS-owned encryption does not create an edge.
				continue
			}

			if out.ServerSideEncryptionConfiguration == nil {
				continue
			}

			for _, rule := range out.ServerSideEncryptionConfiguration.Rules {
				if rule.ApplyServerSideEncryptionByDefault == nil {
					continue
				}

				key := rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID
				if key == nil {
					continue
				}

				add(
					bucket,
					resourceIDFromARN(str(key)),
					"ENCRYPTED_WITH_KMS_KEY",
				)
			}
		}

		// S3 replication -> destination bucket and replication IAM role.
		for _, bucket := range ids["AWS::S3::Bucket"] {
			out, err := r.S3.GetBucketReplication(
				ctx,
				&s3.GetBucketReplicationInput{
					Bucket: &bucket,
				},
			)
			if err != nil {
				continue
			}

			if out.ReplicationConfiguration == nil {
				continue
			}

			if out.ReplicationConfiguration.Role != nil {
				add(
					bucket,
					resourceIDFromARN(str(out.ReplicationConfiguration.Role)),
					"USES_IAM_ROLE",
				)
			}

			for _, rule := range out.ReplicationConfiguration.Rules {
				if rule.Destination == nil || rule.Destination.Bucket == nil {
					continue
				}

				add(
					bucket,
					resourceIDFromARN(str(rule.Destination.Bucket)),
					"REPLICATES_TO_BUCKET",
				)
			}
		}
	}

	// ============================================================
	// Lambda
	// ============================================================

	if r.Lambda != nil {
		for _, function := range ids["AWS::Lambda::Function"] {
			out, err := r.Lambda.GetFunctionConfiguration(
				ctx,
				&lambda.GetFunctionConfigurationInput{
					FunctionName: &function,
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"reading Lambda function %s: %w",
					function,
					err,
				)
			}

			// Lambda -> IAM execution role.
			add(
				function,
				resourceIDFromARN(str(out.Role)),
				"USES_IAM_ROLE",
			)

			// Lambda -> KMS.
			add(
				function,
				resourceIDFromARN(str(out.KMSKeyArn)),
				"USES_KMS_KEY",
			)

			// Lambda -> layers.
			for _, layer := range out.Layers {
				add(
					function,
					resourceIDFromARN(str(layer.Arn)),
					"USES_LAMBDA_LAYER",
				)
			}

			// Lambda -> EFS access points.
			for _, filesystem := range out.FileSystemConfigs {
				add(
					function,
					resourceIDFromARN(str(filesystem.Arn)),
					"USES_EFS_ACCESS_POINT",
				)
			}

			// Lambda -> VPC.
			if out.VpcConfig != nil {
				add(
					function,
					str(out.VpcConfig.VpcId),
					"IN_VPC",
				)

				for _, subnet := range out.VpcConfig.SubnetIds {
					add(
						function,
						subnet,
						"IN_SUBNET",
					)
				}

				for _, group := range out.VpcConfig.SecurityGroupIds {
					add(
						function,
						group,
						"USES_SECURITY_GROUP",
					)
				}
			}

			// Lambda -> dead-letter queue/topic.
			if out.DeadLetterConfig != nil {
				add(
					function,
					resourceIDFromARN(str(out.DeadLetterConfig.TargetArn)),
					"HAS_DEAD_LETTER_DESTINATION",
				)
			}
		}

		// Lambda function -> event source.
		for _, function := range ids["AWS::Lambda::Function"] {
			paginator := lambda.NewListEventSourceMappingsPaginator(
				r.Lambda,
				&lambda.ListEventSourceMappingsInput{
					FunctionName: &function,
				},
			)

			for paginator.HasMorePages() {
				out, err := paginator.NextPage(ctx)
				if err != nil {
					return nil, fmt.Errorf(
						"listing Lambda event source mappings for %s: %w",
						function,
						err,
					)
				}

				for _, mapping := range out.EventSourceMappings {
					sourceARN := str(mapping.EventSourceArn)
					if sourceARN == "" {
						continue
					}

					add(
						function,
						serviceResourceIDFromARN(sourceARN),
						"READS_FROM_EVENT_SOURCE",
					)
				}
			}
		}
	}

	// ============================================================
	// DynamoDB
	// ============================================================

	if r.DynamoDB != nil {
		for _, table := range ids["AWS::DynamoDB::Table"] {
			out, err := r.DynamoDB.DescribeTable(
				ctx,
				&dynamodb.DescribeTableInput{
					TableName: &table,
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"describing DynamoDB table %s: %w",
					table,
					err,
				)
			}

			if out.Table == nil {
				continue
			}

			// DynamoDB table -> KMS key.
			if out.Table.SSEDescription != nil {
				add(
					table,
					resourceIDFromARN(
						str(out.Table.SSEDescription.KMSMasterKeyArn),
					),
					"ENCRYPTED_WITH_KMS_KEY",
				)
			}

			// A DynamoDB stream ARN resolves back to the table identifier in
			// this project's resource-ID scheme. That would create a self-edge,
			// so the stream relationship is not emitted as a graph edge here.

			// Global secondary indexes are logical children of the table and are
			// not separate resources in the inventory represented by state.Resource.

			// Global table replicas expose RegionName rather than a resource ID.
			// Do not create pseudo-nodes for regions in the infrastructure graph.
		}
	}

	// ============================================================
	// RDS
	// ============================================================

	if r.RDS != nil {
		// ------------------------------------------------------------
		// DB instances
		// ------------------------------------------------------------

		// DescribeDBInstances accepts a single DBInstanceIdentifier. Do not
		// pass only the first ID from an arbitrary chunk; query each discovered
		// instance explicitly so every resource is processed.
		for _, instanceID := range ids["AWS::RDS::DBInstance"] {
			out, err := r.RDS.DescribeDBInstances(
				ctx,
				&rds.DescribeDBInstancesInput{
					DBInstanceIdentifier: &instanceID,
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"describing RDS instance %s: %w",
					instanceID,
					err,
				)
			}

			for _, instance := range out.DBInstances {
				id := str(instance.DBInstanceIdentifier)
				if id == "" {
					id = instanceID
				}

				if instance.DBSubnetGroup != nil {
					add(
						id,
						str(instance.DBSubnetGroup.DBSubnetGroupName),
						"USES_DB_SUBNET_GROUP",
					)

					add(
						id,
						str(instance.DBSubnetGroup.VpcId),
						"IN_VPC",
					)
				}

				for _, group := range instance.VpcSecurityGroups {
					add(
						id,
						str(group.VpcSecurityGroupId),
						"USES_SECURITY_GROUP",
					)
				}

				add(
					id,
					str(instance.DBClusterIdentifier),
					"MEMBER_OF_RDS_CLUSTER",
				)

				// Storage encryption.
				add(
					id,
					resourceIDFromARN(str(instance.KmsKeyId)),
					"ENCRYPTED_WITH_KMS_KEY",
				)

				// Performance Insights encryption.
				add(
					id,
					resourceIDFromARN(str(instance.PerformanceInsightsKMSKeyId)),
					"PERFORMANCE_INSIGHTS_ENCRYPTED_WITH_KMS_KEY",
				)

				// Database activity stream encryption.
				add(
					id,
					resourceIDFromARN(str(instance.ActivityStreamKmsKeyId)),
					"ACTIVITY_STREAM_ENCRYPTED_WITH_KMS_KEY",
				)

				for _, parameterGroup := range instance.DBParameterGroups {
					add(
						id,
						str(parameterGroup.DBParameterGroupName),
						"USES_DB_PARAMETER_GROUP",
					)
				}

				for _, optionGroup := range instance.OptionGroupMemberships {
					add(
						id,
						str(optionGroup.OptionGroupName),
						"USES_OPTION_GROUP",
					)
				}

				for _, role := range instance.AssociatedRoles {
					add(
						id,
						resourceIDFromARN(str(role.RoleArn)),
						"USES_IAM_ROLE",
					)
				}

				add(
					id,
					resourceIDFromARN(str(instance.MonitoringRoleArn)),
					"USES_MONITORING_IAM_ROLE",
				)

				if instance.MasterUserSecret != nil {
					add(
						id,
						resourceIDFromARN(str(instance.MasterUserSecret.SecretArn)),
						"USES_MASTER_SECRET",
					)
					add(
						id,
						resourceIDFromARN(str(instance.MasterUserSecret.KmsKeyId)),
						"MASTER_SECRET_ENCRYPTED_WITH_KMS_KEY",
					)
				}

				// Read replica relationships.
				add(
					id,
					str(instance.ReadReplicaSourceDBInstanceIdentifier),
					"REPLICATED_FROM_RDS_INSTANCE",
				)

				for _, replicaID := range instance.ReadReplicaDBInstanceIdentifiers {
					add(
						id,
						replicaID,
						"HAS_READ_REPLICA",
					)
				}

				for _, replicaClusterID := range instance.ReadReplicaDBClusterIdentifiers {
					add(
						id,
						replicaClusterID,
						"HAS_READ_REPLICA_CLUSTER",
					)
				}
			}
		}

		// ------------------------------------------------------------
		// DB clusters
		// ------------------------------------------------------------

		for _, cluster := range ids["AWS::RDS::DBCluster"] {
			out, err := r.RDS.DescribeDBClusters(
				ctx,
				&rds.DescribeDBClustersInput{
					DBClusterIdentifier: &cluster,
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"describing RDS cluster %s: %w",
					cluster,
					err,
				)
			}

			for _, dbCluster := range out.DBClusters {
				clusterID := str(dbCluster.DBClusterIdentifier)
				if clusterID == "" {
					clusterID = cluster
				}

				add(
					clusterID,
					str(dbCluster.DBSubnetGroup),
					"USES_DB_SUBNET_GROUP",
				)

				for _, group := range dbCluster.VpcSecurityGroups {
					add(
						clusterID,
						str(group.VpcSecurityGroupId),
						"USES_SECURITY_GROUP",
					)
				}

				add(
					clusterID,
					resourceIDFromARN(str(dbCluster.KmsKeyId)),
					"ENCRYPTED_WITH_KMS_KEY",
				)

				add(
					clusterID,
					resourceIDFromARN(str(dbCluster.PerformanceInsightsKMSKeyId)),
					"PERFORMANCE_INSIGHTS_ENCRYPTED_WITH_KMS_KEY",
				)

				add(
					clusterID,
					resourceIDFromARN(str(dbCluster.ActivityStreamKmsKeyId)),
					"ACTIVITY_STREAM_ENCRYPTED_WITH_KMS_KEY",
				)

				add(
					clusterID,
					str(dbCluster.DBClusterParameterGroup),
					"USES_DB_CLUSTER_PARAMETER_GROUP",
				)

				for _, optionGroup := range dbCluster.DBClusterOptionGroupMemberships {
					add(
						clusterID,
						str(optionGroup.DBClusterOptionGroupName),
						"USES_CLUSTER_OPTION_GROUP",
					)
				}

				for _, role := range dbCluster.AssociatedRoles {
					add(
						clusterID,
						resourceIDFromARN(str(role.RoleArn)),
						"USES_IAM_ROLE",
					)
				}

				add(
					clusterID,
					resourceIDFromARN(str(dbCluster.MonitoringRoleArn)),
					"USES_MONITORING_IAM_ROLE",
				)

				if dbCluster.MasterUserSecret != nil {
					add(
						clusterID,
						resourceIDFromARN(str(dbCluster.MasterUserSecret.SecretArn)),
						"USES_MASTER_SECRET",
					)
					add(
						clusterID,
						resourceIDFromARN(str(dbCluster.MasterUserSecret.KmsKeyId)),
						"MASTER_SECRET_ENCRYPTED_WITH_KMS_KEY",
					)
				}

				for _, instance := range dbCluster.DBClusterMembers {
					add(
						clusterID,
						str(instance.DBInstanceIdentifier),
						"CONTAINS_RDS_INSTANCE",
					)
				}

				for _, replicaID := range dbCluster.ReadReplicaIdentifiers {
					add(
						clusterID,
						replicaID,
						"HAS_READ_REPLICA_CLUSTER",
					)
				}

				add(
					clusterID,
					str(dbCluster.ReplicationSourceIdentifier),
					"REPLICATED_FROM_RDS_SOURCE",
				)
			}
		}

		// ------------------------------------------------------------
		// DB subnet groups
		// ------------------------------------------------------------

		for _, subnetGroup := range ids["AWS::RDS::DBSubnetGroup"] {
			out, err := r.RDS.DescribeDBSubnetGroups(
				ctx,
				&rds.DescribeDBSubnetGroupsInput{
					DBSubnetGroupName: &subnetGroup,
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"describing RDS DB subnet group %s: %w",
					subnetGroup,
					err,
				)
			}

			for _, group := range out.DBSubnetGroups {
				groupID := str(group.DBSubnetGroupName)
				if groupID == "" {
					groupID = subnetGroup
				}

				add(
					groupID,
					str(group.VpcId),
					"IN_VPC",
				)

				for _, subnet := range group.Subnets {
					add(
						groupID,
						str(subnet.SubnetIdentifier),
						"CONTAINS_SUBNET",
					)
				}
			}
		}
	}

	return deduplicateEdges(edges), nil
}

// serviceResourceIDFromARN extracts the useful resource identity from common
// event-source ARNs. This is intentionally conservative.
func serviceResourceIDFromARN(arn string) string {
	if arn == "" {
		return ""
	}

	parts := strings.Split(arn, ":")

	if len(parts) < 6 {
		return ""
	}

	resource := parts[5]

	switch {
	case strings.HasPrefix(resource, "table/"):
		// DynamoDB stream:
		// table/MyTable/stream/2026-...
		resource = strings.TrimPrefix(resource, "table/")
		if index := strings.Index(resource, "/"); index >= 0 {
			return resource[:index]
		}
		return resource

	case strings.HasPrefix(resource, "queue/"):
		return strings.TrimPrefix(resource, "queue/")

	case strings.HasPrefix(resource, "stream/"):
		return strings.TrimPrefix(resource, "stream/")

	case strings.HasPrefix(resource, "function:"):
		return strings.TrimPrefix(resource, "function:")

	default:
		return resource
	}
}
