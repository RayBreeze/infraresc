package aws

import "infraresc/state"

func mapResource(
	id string,
	resourceType string,
	name string,
	region string,
	configuration map[string]any,
	dependencies []string,
) state.Resource {
	return state.Resource{
		ID:            id,
		Type:          resourceType,
		Name:          name,
		Region:        region,
		Configuration: configuration,
		Dependencies:  dependencies,
	}
}
