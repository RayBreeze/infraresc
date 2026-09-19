package recover

import (
	"context"
	"fmt"

	"infraresc/state"
)

func (e *Engine) recoverResource(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	switch item.ResourceType {

	case "AWS::EC2::Instance":
		return e.recoverEC2(
			ctx,
			snapshot,
			item,
		)

	case "AWS::EC2::Subnet":
		return e.recoverSubnet(
			ctx,
			snapshot,
			item,
		)

	case "AWS::EC2::SecurityGroup":
		return e.recoverSecurityGroup(
			ctx,
			snapshot,
			item,
		)

	case "AWS::EC2::VPC":
		return e.recoverVPC(
			ctx,
			snapshot,
			item,
		)

	case "AWS::S3::Bucket":
		return e.recoverS3(
			ctx,
			snapshot,
			item,
		)

	case "AWS::Lambda::Function":
		return e.recoverLambda(
			ctx,
			snapshot,
			item,
		)

	case "AWS::DynamoDB::Table":
		return e.recoverDynamoDB(
			ctx,
			snapshot,
			item,
		)

	case "AWS::RDS::DBInstance":
		return e.recoverRDSInstance(
			ctx,
			snapshot,
			item,
		)

	case "AWS::RDS::DBCluster":
		return e.recoverRDSCluster(
			ctx,
			snapshot,
			item,
		)

	default:
		return fmt.Errorf(
			"recovery not implemented for resource type %q",
			item.ResourceType,
		)
	}
}
