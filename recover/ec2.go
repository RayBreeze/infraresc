package recover

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"infraresc/state"
)

func (e *Engine) recoverVPC(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	config, ok := findConfig(
		snapshot,
		item.ResourceID,
	)

	if !ok {
		return fmt.Errorf(
			"no configuration captured for %s",
			item.ResourceID,
		)
	}

	cidr := stringProperty(
		config.Properties,
		"CidrBlock",
	)

	if cidr == "" {
		cidr = stringProperty(
			config.Properties,
			"cidr_block",
		)
	}

	if cidr == "" {
		return fmt.Errorf(
			"VPC %s has no CIDR block in snapshot",
			item.ResourceID,
		)
	}

	_, err := e.AWS.EC2.CreateVpc(
		ctx,
		&ec2.CreateVpcInput{
			CidrBlock: aws.String(cidr),
		},
	)

	if err != nil {
		return fmt.Errorf(
			"create VPC %s: %w",
			item.ResourceID,
			err,
		)
	}

	return nil
}

func (e *Engine) recoverSubnet(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	config, ok := findConfig(
		snapshot,
		item.ResourceID,
	)

	if !ok {
		return fmt.Errorf(
			"no configuration captured for %s",
			item.ResourceID,
		)
	}

	vpcID := stringProperty(
		config.Properties,
		"VpcId",
	)

	cidr := stringProperty(
		config.Properties,
		"CidrBlock",
	)

	if vpcID == "" || cidr == "" {
		return fmt.Errorf(
			"subnet %s lacks VPC ID or CIDR",
			item.ResourceID,
		)
	}

	input := &ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcID),
		CidrBlock: aws.String(cidr),
	}

	if az := stringProperty(
		config.Properties,
		"AvailabilityZone",
	); az != "" {
		input.AvailabilityZone = aws.String(az)
	}

	_, err := e.AWS.EC2.CreateSubnet(
		ctx,
		input,
	)

	if err != nil {
		return fmt.Errorf(
			"create subnet %s: %w",
			item.ResourceID,
			err,
		)
	}

	return nil
}

func (e *Engine) recoverSecurityGroup(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	config, ok := findConfig(
		snapshot,
		item.ResourceID,
	)

	if !ok {
		return fmt.Errorf(
			"no configuration captured for %s",
			item.ResourceID,
		)
	}

	name := stringProperty(
		config.Properties,
		"GroupName",
	)

	description := stringProperty(
		config.Properties,
		"Description",
	)

	vpcID := stringProperty(
		config.Properties,
		"VpcId",
	)

	if name == "" {
		name = "infraresc-recovered"
	}

	if description == "" {
		description = "Recovered by InfraResc"
	}

	_, err := e.AWS.EC2.CreateSecurityGroup(
		ctx,
		&ec2.CreateSecurityGroupInput{
			GroupName:   aws.String(name),
			Description: aws.String(description),
			VpcId:       aws.String(vpcID),
		},
	)

	if err != nil {
		return fmt.Errorf(
			"create security group %s: %w",
			item.ResourceID,
			err,
		)
	}

	return nil
}

func (e *Engine) recoverEC2(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	_ = ctx
	_ = snapshot

	return fmt.Errorf(
		"EC2 instance recovery requires an AMI/image reference and launch configuration; resource %s is not currently recoverable from the captured snapshot",
		item.ResourceID,
	)
}

func stringProperty(
	properties map[string]interface{},
	key string,
) string {

	value, ok := properties[key]

	if !ok {
		return ""
	}

	switch value := value.(type) {

	case string:
		return strings.TrimSpace(value)

	default:
		return fmt.Sprint(value)
	}
}

func findConfig(
	snapshot *state.Snapshot,
	resourceID string,
) (state.ResourceConfig, bool) {

	for _, config := range snapshot.Configs {

		if config.ResourceID == resourceID {
			return config, true
		}
	}

	return state.ResourceConfig{}, false
}
