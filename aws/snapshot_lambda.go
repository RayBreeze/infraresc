package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"infraresc/state"
)

func (s *SnapshotCollector) collectLambda(
	ctx context.Context,
	resources []state.Resource,
) ([]state.ResourceConfig, []state.SnapshotWarning) {

	if s.client.Lambda == nil {
		return nil, []state.SnapshotWarning{
			{
				Type:    "AWS::Lambda",
				Message: "Lambda client is not initialized",
			},
		}
	}

	var configs []state.ResourceConfig
	var warnings []state.SnapshotWarning

	for _, resource := range snapshotResourceType(
		resources,
		"AWS::Lambda::Function",
	) {

		out, err := s.client.Lambda.GetFunction(
			ctx,
			&lambda.GetFunctionInput{
				FunctionName: &resource.ID,
			},
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		// GetFunction contains both configuration and deployment
		// metadata. We deliberately keep the response as JSON so
		// Stage 3 can decide what is actually required for recovery.
		config, err := configFromValue(
			resource,
			out,
		)

		if err != nil {
			warnings = append(
				warnings,
				snapshotWarning(resource, err),
			)
			continue
		}

		configs = append(configs, config)

		// --------------------------------------------------------
		// Concurrency
		// --------------------------------------------------------

		concurrency, err := s.client.Lambda.GetFunctionConcurrency(
			ctx,
			&lambda.GetFunctionConcurrencyInput{
				FunctionName: &resource.ID,
			},
		)

		if err == nil {
			if config, err := configFromValue(
				resource,
				concurrency,
			); err == nil {

				config.Properties["snapshot_component"] =
					"concurrency"

				configs = append(configs, config)
			}
		}

		// --------------------------------------------------------
		// Event source mappings
		// --------------------------------------------------------

		paginator := lambda.NewListEventSourceMappingsPaginator(
			s.client.Lambda,
			&lambda.ListEventSourceMappingsInput{
				FunctionName: &resource.ID,
			},
		)

		for paginator.HasMorePages() {

			page, err := paginator.NextPage(ctx)

			if err != nil {
				warnings = append(
					warnings,
					snapshotWarning(resource, err),
				)
				break
			}

			for _, mapping := range page.EventSourceMappings {

				config, err := configFromValue(
					resource,
					mapping,
				)

				if err != nil {
					warnings = append(
						warnings,
						snapshotWarning(resource, err),
					)
					continue
				}

				config.Properties["snapshot_component"] =
					"event_source_mapping"

				configs = append(configs, config)
			}
		}
	}

	return configs, warnings
}
