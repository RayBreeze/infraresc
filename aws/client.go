package aws

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/resourceexplorer2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type Client struct {
	Config           awsv2.Config
	STS              *sts.Client
	EC2              *ec2.Client
	S3               *s3.Client
	Lambda           *lambda.Client
	DynamoDB         *dynamodb.Client
	RDS              *rds.Client
	ResourceExplorer *resourceexplorer2.Client
}

type CallerIdentity struct {
	AccountID    string
	PrincipalARN string
	UserID       string
	Region       string
}

func NewClient(
	ctx context.Context,
	profile string,
) (*Client, error) {

	loadOptions := []func(*config.LoadOptions) error{}

	if profile != "" {
		loadOptions = append(
			loadOptions,
			config.WithSharedConfigProfile(profile),
		)
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		loadOptions...,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to load AWS configuration: %w",
			err,
		)
	}

	return &Client{
		Config:           cfg,
		STS:              sts.NewFromConfig(cfg),
		EC2:              ec2.NewFromConfig(cfg),
		S3:               s3.NewFromConfig(cfg),
		Lambda:           lambda.NewFromConfig(cfg),
		DynamoDB:         dynamodb.NewFromConfig(cfg),
		RDS:              rds.NewFromConfig(cfg),
		ResourceExplorer: resourceexplorer2.NewFromConfig(cfg),
	}, nil
}

func (c *Client) Identity(
	ctx context.Context,
) (*CallerIdentity, error) {

	result, err := c.STS.GetCallerIdentity(
		ctx,
		&sts.GetCallerIdentityInput{},
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to verify AWS identity: %w",
			err,
		)
	}

	return &CallerIdentity{
		AccountID:    stringValue(result.Account),
		PrincipalARN: stringValue(result.Arn),
		UserID:       stringValue(result.UserId),
		Region:       c.Config.Region,
	}, nil
}

func ProfileRegion(
	ctx context.Context,
	cliPath string,
	profile string,
) (string, error) {

	if cliPath == "" {
		cliPath = "aws"
	}

	cmd := exec.CommandContext(
		ctx,
		cliPath,
		"configure",
		"get",
		"region",
		"--profile",
		profile,
	)

	output, err := cmd.Output()

	if err != nil {
		return "", fmt.Errorf(
			"failed to determine AWS region: %w",
			err,
		)
	}

	region := strings.TrimSpace(string(output))

	if region == "" {
		return "", fmt.Errorf(
			"AWS region is not configured for profile %q",
			profile,
		)
	}

	return region, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
