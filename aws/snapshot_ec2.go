package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"infraresc/state"
)

func (s *SnapshotCollector) collectEC2(
	ctx context.Context,
	resources []state.Resource,
) ([]state.ResourceConfig, []state.SnapshotWarning) {

	if s.client.EC2 == nil {
		return nil, []state.SnapshotWarning{
			{
				Type:    "AWS::EC2",
				Message: "EC2 client is not initialized",
			},
		}
	}

	var configs []state.ResourceConfig
	var warnings []state.SnapshotWarning

	// ------------------------------------------------------------
	// EC2 Instances
	// ------------------------------------------------------------

	instances := snapshotResourceType(
		resources,
		"AWS::EC2::Instance",
	)

	// AWS API responses use native instance IDs, so keep a native-ID
	// lookup map even though Resource.ID is now the canonical ARN.
	instanceMap := make(map[string]state.Resource, len(instances))

	var instanceIDs []string

	for _, resource := range instances {
		apiID := resourceIDFromARN(resource.ARN)

		instanceIDs = append(
			instanceIDs,
			apiID,
		)

		instanceMap[apiID] = resource
	}

	for _, batch := range chunk(instanceIDs, 100) {

		out, err := s.client.EC2.DescribeInstances(
			ctx,
			&ec2.DescribeInstancesInput{
				InstanceIds: batch,
			},
		)

		if err != nil {
			for _, id := range batch {
				resource, exists := instanceMap[id]
				if !exists {
					continue
				}

				warnings = append(
					warnings,
					snapshotWarning(resource, fmt.Errorf(
						"describing EC2 instance: %w",
						err,
					)),
				)
			}

			continue
		}

		for _, reservation := range out.Reservations {
			for _, instance := range reservation.Instances {

				id := str(instance.InstanceId)

				resource, exists := instanceMap[id]
				if !exists {
					continue
				}

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
	}

	// ------------------------------------------------------------
	// Subnets
	// ------------------------------------------------------------

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::EC2::Subnet",
	) {

		apiID := resourceIDFromARN(resource.ARN)

		out, err := s.client.EC2.DescribeSubnets(
			ctx,
			&ec2.DescribeSubnetsInput{
				SubnetIds: []string{apiID},
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		for _, subnet := range out.Subnets {

			config, err := configFromValue(
				resource,
				subnet,
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
	// Security Groups
	// ------------------------------------------------------------

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::EC2::SecurityGroup",
	) {

		apiID := resourceIDFromARN(resource.ARN)

		out, err := s.client.EC2.DescribeSecurityGroups(
			ctx,
			&ec2.DescribeSecurityGroupsInput{
				GroupIds: []string{apiID},
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		for _, group := range out.SecurityGroups {

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

	// ------------------------------------------------------------
	// VPCs
	// ------------------------------------------------------------

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::EC2::VPC",
	) {

		apiID := resourceIDFromARN(resource.ARN)

		out, err := s.client.EC2.DescribeVpcs(
			ctx,
			&ec2.DescribeVpcsInput{
				VpcIds: []string{apiID},
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		for _, vpc := range out.Vpcs {

			config, err := configFromValue(
				resource,
				vpc,
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
