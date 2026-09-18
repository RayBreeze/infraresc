package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/configservice"
)

type Discovery struct {
	Config *configservice.Client
}

func NewDiscovery(client *Client) *Discovery {
	return &Discovery{
		Config: configservice.NewFromConfig(client.Config),
	}
}

func (d *Discovery) CheckRecorder(
	ctx context.Context,
) error {

	output, err := d.Config.DescribeConfigurationRecorderStatus(
		ctx,
		&configservice.DescribeConfigurationRecorderStatusInput{},
	)

	if err != nil {
		return fmt.Errorf(
			"checking AWS Config recorder status: %w",
			err,
		)
	}

	if len(output.ConfigurationRecordersStatus) == 0 {
		return fmt.Errorf(
			"no AWS Config recorder found",
		)
	}

	for _, recorder := range output.ConfigurationRecordersStatus {
		if !recorder.Recording {
			return fmt.Errorf(
				"AWS Config recorder %q is not recording",
				*recorder.Name,
			)
		}
	}

	return nil
}

func (d *Discovery) CheckRecorderConfiguration(
	ctx context.Context,
) error {

	output, err := d.Config.DescribeConfigurationRecorders(
		ctx,
		&configservice.DescribeConfigurationRecordersInput{},
	)

	if err != nil {
		return fmt.Errorf(
			"checking AWS Config recorder configuration: %w",
			err,
		)
	}

	if len(output.ConfigurationRecorders) == 0 {
		return fmt.Errorf(
			"no AWS Config configuration recorder found",
		)
	}

	return nil
}

func (d *Discovery) Resources(
	ctx context.Context,
) ([]string, error) {

	query := `
SELECT
    resourceId,
    resourceType,
    resourceName,
    awsRegion,
    configuration
WHERE
    resourceId IS NOT NULL
`

	var resources []string

	var nextToken *string

	for {
		input := &configservice.SelectResourceConfigInput{
			Expression: &query,
			NextToken:  nextToken,
		}

		output, err := d.Config.SelectResourceConfig(
			ctx,
			input,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"querying AWS Config resources: %w",
				err,
			)
		}

		resources = append(
			resources,
			output.Results...,
		)

		if output.NextToken == nil ||
			*output.NextToken == "" {
			break
		}

		nextToken = output.NextToken
	}

	return resources, nil
}
