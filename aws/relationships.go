package aws

import (
	"context"
	"fmt"

	"infraresc/state"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// RelationshipDiscovery builds the infrastructure graph by asking the
// resource-specific AWS APIs for relationships that Resource Explorer does
// not expose as graph edges.
//
// The current implementation focuses on the core EC2/VPC resource family.
// It deliberately keeps discovery relationship-only: configuration belongs
// in the later snapshot stage.
type RelationshipDiscovery struct {
	EC2      *ec2.Client
	S3       *s3.Client
	Lambda   *lambda.Client
	DynamoDB *dynamodb.Client
	RDS      *rds.Client
}

func NewRelationshipDiscovery(client *Client) *RelationshipDiscovery {
	return &RelationshipDiscovery{
		EC2:      client.EC2,
		S3:       client.S3,
		Lambda:   client.Lambda,
		DynamoDB: client.DynamoDB,
		RDS:      client.RDS,
	}
}

// Discover returns directed relationships between resources already found by
// Resource Explorer. Missing resources are simply ignored; this keeps the
// graph consistent when a service/API does not expose a particular object.
func (r *RelationshipDiscovery) Discover(
	ctx context.Context,
	resources []state.Resource,
) ([]state.Edge, error) {
	if r.EC2 == nil {
		return nil, fmt.Errorf("EC2 client is not initialized")
	}

	ids := idsByType(resources)

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

	// EC2 instances
	for _, batch := range chunk(ids["AWS::EC2::Instance"], 100) {
		out, err := r.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing EC2 instances: %w", err)
		}

		for _, reservation := range out.Reservations {
			for _, instance := range reservation.Instances {
				instanceID := str(instance.InstanceId)
				if instanceID == "" {
					continue
				}

				add(instanceID, str(instance.SubnetId), "IN_SUBNET")
				add(instanceID, str(instance.VpcId), "IN_VPC")

				for _, group := range instance.SecurityGroups {
					add(instanceID, str(group.GroupId), "USES_SECURITY_GROUP")
				}

				for _, eni := range instance.NetworkInterfaces {
					add(instanceID, str(eni.NetworkInterfaceId), "HAS_NETWORK_INTERFACE")
				}

				for _, mapping := range instance.BlockDeviceMappings {
					if mapping.Ebs != nil {
						add(instanceID, str(mapping.Ebs.VolumeId), "ATTACHED_VOLUME")
					}
				}
			}
		}
	}

	// Subnets -> VPCs
	for _, batch := range chunk(ids["AWS::EC2::Subnet"], 100) {
		out, err := r.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			SubnetIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing subnets: %w", err)
		}

		for _, subnet := range out.Subnets {
			add(str(subnet.SubnetId), str(subnet.VpcId), "BELONGS_TO_VPC")
		}
	}

	// Route tables -> VPCs, subnets, NAT gateways and internet gateways.
	for _, batch := range chunk(ids["AWS::EC2::RouteTable"], 100) {
		out, err := r.EC2.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
			RouteTableIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing route tables: %w", err)
		}

		for _, table := range out.RouteTables {
			tableID := str(table.RouteTableId)
			add(tableID, str(table.VpcId), "BELONGS_TO_VPC")

			for _, association := range table.Associations {
				add(tableID, str(association.SubnetId), "ASSOCIATED_WITH_SUBNET")
			}

			for _, route := range table.Routes {
				add(tableID, str(route.NatGatewayId), "ROUTES_THROUGH_NAT")
				add(tableID, str(route.GatewayId), "ROUTES_THROUGH_GATEWAY")
				add(tableID, str(route.NetworkInterfaceId), "ROUTES_TO_NETWORK_INTERFACE")
				add(tableID, str(route.InstanceId), "ROUTES_TO_INSTANCE")
			}
		}
	}

	// Internet gateways -> VPCs.
	for _, batch := range chunk(ids["AWS::EC2::InternetGateway"], 100) {
		out, err := r.EC2.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
			InternetGatewayIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing internet gateways: %w", err)
		}

		for _, gateway := range out.InternetGateways {
			gatewayID := str(gateway.InternetGatewayId)

			for _, attachment := range gateway.Attachments {
				add(gatewayID, str(attachment.VpcId), "ATTACHED_TO_VPC")
			}
		}
	}

	// NAT gateways -> subnet, VPC, and the NAT ENI/EIP when exposed.
	for _, batch := range chunk(ids["AWS::EC2::NatGateway"], 100) {
		out, err := r.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
			NatGatewayIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing NAT gateways: %w", err)
		}

		for _, gateway := range out.NatGateways {
			gatewayID := str(gateway.NatGatewayId)

			add(gatewayID, str(gateway.SubnetId), "IN_SUBNET")
			add(gatewayID, str(gateway.VpcId), "IN_VPC")

			for _, address := range gateway.NatGatewayAddresses {
				add(gatewayID, str(address.NetworkInterfaceId), "HAS_NETWORK_INTERFACE")
				add(gatewayID, str(address.AllocationId), "USES_ELASTIC_IP")
			}
		}
	}

	// Security groups -> VPC.
	for _, batch := range chunk(ids["AWS::EC2::SecurityGroup"], 100) {
		out, err := r.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			GroupIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing security groups: %w", err)
		}

		for _, group := range out.SecurityGroups {
			add(str(group.GroupId), str(group.VpcId), "BELONGS_TO_VPC")

			// Security-group-to-security-group references are useful for
			// recovery ordering and are available in ingress/egress rules.
			for _, permission := range group.IpPermissions {
				for _, referenced := range permission.UserIdGroupPairs {
					add(str(group.GroupId), str(referenced.GroupId), "REFERENCES_SECURITY_GROUP")
				}
			}

			for _, permission := range group.IpPermissionsEgress {
				for _, referenced := range permission.UserIdGroupPairs {
					add(str(group.GroupId), str(referenced.GroupId), "REFERENCES_SECURITY_GROUP")
				}
			}
		}
	}

	// Network interfaces -> subnet, VPC, security groups and instance.
	for _, batch := range chunk(ids["AWS::EC2::NetworkInterface"], 100) {
		out, err := r.EC2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing network interfaces: %w", err)
		}

		for _, eni := range out.NetworkInterfaces {
			eniID := str(eni.NetworkInterfaceId)

			add(eniID, str(eni.SubnetId), "IN_SUBNET")
			add(eniID, str(eni.VpcId), "IN_VPC")

			for _, group := range eni.Groups {
				add(eniID, str(group.GroupId), "USES_SECURITY_GROUP")
			}

			if eni.Attachment != nil {
				add(eniID, str(eni.Attachment.InstanceId), "ATTACHED_TO_INSTANCE")
			}

			if eni.Association != nil {
				add(eniID, str(eni.Association.AllocationId), "USES_ELASTIC_IP")
			}
		}
	}

	// EBS volumes -> EC2 instances.
	for _, batch := range chunk(ids["AWS::EC2::Volume"], 100) {
		out, err := r.EC2.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing EBS volumes: %w", err)
		}

		for _, volume := range out.Volumes {
			volumeID := str(volume.VolumeId)

			for _, attachment := range volume.Attachments {
				add(volumeID, str(attachment.InstanceId), "ATTACHED_TO_INSTANCE")
			}

			add(
				volumeID,
				str(volume.SnapshotId),
				"CREATED_FROM_SNAPSHOT",
			)

			add(
				volumeID,
				str(volume.KmsKeyId),
				"ENCRYPTED_WITH_KMS_KEY",
			)
		}
	}

	// Elastic IPs -> instance/ENI.
	for _, batch := range chunk(ids["AWS::EC2::EIP"], 100) {
		out, err := r.EC2.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
			AllocationIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing elastic IPs: %w", err)
		}

		for _, address := range out.Addresses {
			eipID := str(address.AllocationId)
			add(eipID, str(address.InstanceId), "ASSOCIATED_WITH_INSTANCE")
			add(eipID, str(address.NetworkInterfaceId), "ASSOCIATED_WITH_NETWORK_INTERFACE")
		}
	}

	// EBS snapshots -> source EBS volume.
	for _, batch := range chunk(ids["AWS::EC2::Snapshot"], 100) {
		out, err := r.EC2.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
			SnapshotIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing EBS snapshots: %w", err)
		}

		for _, snapshot := range out.Snapshots {
			add(str(snapshot.SnapshotId), str(snapshot.VolumeId), "CREATED_FROM_VOLUME")
		}
	}

	// VPCs -> DHCP options.
	for _, batch := range chunk(ids["AWS::EC2::VPC"], 100) {
		out, err := r.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			VpcIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing VPCs: %w", err)
		}

		for _, vpc := range out.Vpcs {
			vpcID := str(vpc.VpcId)

			add(vpcID, str(vpc.DhcpOptionsId), "USES_DHCP_OPTIONS")
		}
	}

	// Network ACLs -> VPC and subnets.
	for _, batch := range chunk(ids["AWS::EC2::NetworkAcl"], 100) {
		out, err := r.EC2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{
			NetworkAclIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing network ACLs: %w", err)
		}

		for _, acl := range out.NetworkAcls {
			aclID := str(acl.NetworkAclId)

			add(aclID, str(acl.VpcId), "BELONGS_TO_VPC")

			for _, association := range acl.Associations {
				add(
					aclID,
					str(association.SubnetId),
					"ASSOCIATED_WITH_SUBNET",
				)
			}
		}
	}

	// Egress-only internet gateways -> VPC.
	for _, batch := range chunk(ids["AWS::EC2::EgressOnlyInternetGateway"], 100) {
		out, err := r.EC2.DescribeEgressOnlyInternetGateways(
			ctx,
			&ec2.DescribeEgressOnlyInternetGatewaysInput{
				EgressOnlyInternetGatewayIds: batch,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"describing egress-only internet gateways: %w",
				err,
			)
		}

		for _, gateway := range out.EgressOnlyInternetGateways {
			gatewayID := str(gateway.EgressOnlyInternetGatewayId)

			if gateway.Attachments == nil {
				continue
			}

			for _, attachment := range gateway.Attachments {
				add(
					gatewayID,
					str(attachment.VpcId),
					"ATTACHED_TO_VPC",
				)
			}
		}
	}

	// VPC endpoints -> VPC, subnets, route tables, security groups and ENIs.
	for _, batch := range chunk(ids["AWS::EC2::VPCEndpoint"], 100) {
		out, err := r.EC2.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			VpcEndpointIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing VPC endpoints: %w", err)
		}

		for _, endpoint := range out.VpcEndpoints {
			endpointID := str(endpoint.VpcEndpointId)

			add(endpointID, str(endpoint.VpcId), "BELONGS_TO_VPC")

			for _, subnetID := range endpoint.SubnetIds {
				add(endpointID, subnetID, "IN_SUBNET")
			}

			for _, routeTableID := range endpoint.RouteTableIds {
				add(
					endpointID,
					routeTableID,
					"ASSOCIATED_WITH_ROUTE_TABLE",
				)
			}

			for _, group := range endpoint.Groups {
				add(
					endpointID,
					str(group.GroupId),
					"USES_SECURITY_GROUP",
				)
			}

			for _, eniID := range endpoint.NetworkInterfaceIds {
				add(
					endpointID,
					eniID,
					"HAS_NETWORK_INTERFACE",
				)
			}
		}
	}

	// VPC flow logs -> their source resource.
	// ResourceId may refer to VPC, subnet or network interface.
	for _, batch := range chunk(ids["AWS::EC2::FlowLog"], 100) {
		out, err := r.EC2.DescribeFlowLogs(ctx, &ec2.DescribeFlowLogsInput{
			FlowLogIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing flow logs: %w", err)
		}

		for _, flowLog := range out.FlowLogs {
			flowLogID := str(flowLog.FlowLogId)

			add(
				flowLogID,
				str(flowLog.ResourceId),
				"CAPTURES_TRAFFIC_FROM",
			)
		}
	}

	// VPC peering -> both participating VPCs.
	for _, batch := range chunk(ids["AWS::EC2::VPCPeeringConnection"], 100) {
		out, err := r.EC2.DescribeVpcPeeringConnections(ctx, &ec2.DescribeVpcPeeringConnectionsInput{
			VpcPeeringConnectionIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("describing VPC peering connections: %w", err)
		}

		for _, peering := range out.VpcPeeringConnections {
			peeringID := str(peering.VpcPeeringConnectionId)

			if peering.RequesterVpcInfo != nil {
				add(peeringID, str(peering.RequesterVpcInfo.VpcId), "CONNECTS_VPC")
			}
			if peering.AccepterVpcInfo != nil {
				add(peeringID, str(peering.AccepterVpcInfo.VpcId), "CONNECTS_VPC")
			}
		}
	}

	serviceEdges, err := r.discoverServiceRelationships(ctx, ids)
	if err != nil {
		return nil, err
	}

	edges = append(edges, serviceEdges...)

	return deduplicateEdges(edges), nil
}

func idsByType(resources []state.Resource) map[string][]string {
	result := make(map[string][]string)

	for _, resource := range resources {
		if resource.ID == "" || resource.Type == "" {
			continue
		}

		result[resource.Type] = append(result[resource.Type], resource.ID)
	}

	return result
}

func chunk(values []string, size int) [][]string {
	if len(values) == 0 {
		return nil
	}
	if size <= 0 {
		size = 100
	}

	var result [][]string

	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}

		result = append(result, values[start:end])
	}

	return result
}

func deduplicateEdges(edges []state.Edge) []state.Edge {
	seen := make(map[string]struct{}, len(edges))
	result := make([]state.Edge, 0, len(edges))

	for _, edge := range edges {
		key := edge.From + "\x00" + edge.To + "\x00" + edge.Relation

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, edge)
	}

	return result
}

func str(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
