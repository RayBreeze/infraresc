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
				// Not every bucket has explicit encryption configuration.
				// AWS-owned encryption is therefore not treated as an error.
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

				if key != nil {
					add(
						bucket,
						resourceIDFromARN(str(key)),
						"ENCRYPTED_WITH_KMS_KEY",
					)
				}
			}
		}

		// S3 replication -> destination bucket.
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

			for _, rule := range out.ReplicationConfiguration.Rules {
				if rule.Destination == nil {
					continue
				}

				destination := rule.Destination.Bucket

				if destination != nil {
					add(
						bucket,
						resourceIDFromARN(str(destination)),
						"REPLICATES_TO_BUCKET",
					)
				}
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

			// Lambda -> dead-letter queue.
			if out.DeadLetterConfig != nil {
				add(
					function,
					resourceIDFromARN(str(out.DeadLetterConfig.TargetArn)),
					"HAS_DEAD_LETTER_DESTINATION",
				)
			}
		}

		// Lambda Event Source Mapping -> event source.
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

			// DynamoDB table -> KMS.
			if out.Table.SSEDescription != nil {
				add(
					table,
					resourceIDFromARN(
						str(out.Table.SSEDescription.KMSMasterKeyArn),
					),
					"ENCRYPTED_WITH_KMS_KEY",
				)
			}

			// DynamoDB table -> stream.
			if out.Table.LatestStreamArn != nil {
				add(
					table,
					resourceIDFromARN(str(out.Table.LatestStreamArn)),
					"HAS_STREAM",
				)
			}

			// Global secondary indexes.
			for _, index := range out.Table.GlobalSecondaryIndexes {
				// The index is not necessarily discovered as a separate
				// Resource Explorer resource, so this edge is intentionally
				// omitted from the graph unless the index itself exists.
				_ = index
			}

			// Global table replicas.
			for _, replica := range out.Table.Replicas {
				add(
					table,
					str(replica.RegionName),
					"REPLICATED_TO_REGION",
				)
			}
		}
	}

	// ============================================================
	// RDS
	// ============================================================

	if r.RDS != nil {
		// RDS instances -> DB subnet group / VPC / security groups.
		for _, batch := range chunk(ids["AWS::RDS::DBInstance"], 100) {
			out, err := r.RDS.DescribeDBInstances(
				ctx,
				&rds.DescribeDBInstancesInput{
					DBInstanceIdentifier: &batch[0],
				},
			)
			if err != nil {
				return nil, fmt.Errorf(
					"describing RDS instance %s: %w",
					batch[0],
					err,
				)
			}

			for _, instance := range out.DBInstances {
				instanceID := str(instance.DBInstanceIdentifier)

				if instance.DBSubnetGroup != nil {
					add(
						instanceID,
						str(instance.DBSubnetGroup.DBSubnetGroupName),
						"USES_DB_SUBNET_GROUP",
					)

					add(
						instanceID,
						str(instance.DBSubnetGroup.VpcId),
						"IN_VPC",
					)
				}

				for _, group := range instance.VpcSecurityGroups {
					add(
						instanceID,
						str(group.VpcSecurityGroupId),
						"USES_SECURITY_GROUP",
					)
				}

				add(
					instanceID,
					str(instance.DBClusterIdentifier),
					"MEMBER_OF_RDS_CLUSTER",
				)
			}
		}

		// RDS clusters -> subnet group / security groups / instances.
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

				for _, instance := range dbCluster.DBClusterMembers {
					add(
						clusterID,
						str(instance.DBInstanceIdentifier),
						"CONTAINS_RDS_INSTANCE",
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
