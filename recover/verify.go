package recover

import (
	"encoding/json"
	"reflect"
	"sort"

	"infraresc/state"
)

type ChangeSet struct {
	Missing []state.Resource
	Changed []state.Resource
	Live    *state.Snapshot
}

func Compare(
	snapshot *state.Snapshot,
	live *state.Snapshot,
) *ChangeSet {

	result := &ChangeSet{
		Live: live,
	}

	liveResources := make(map[string]state.Resource)

	for _, resource := range live.Resources {
		liveResources[resource.ID] = resource
	}

	liveConfigs := configMap(live.Configs)

	for _, resource := range snapshot.Resources {

		liveResource, exists := liveResources[resource.ID]

		if !exists {
			result.Missing = append(
				result.Missing,
				resource,
			)

			continue
		}

		if resource.Type != liveResource.Type ||
			resource.Service != liveResource.Service ||
			resource.Region != liveResource.Region {

			result.Changed = append(
				result.Changed,
				resource,
			)

			continue
		}

		if configsChanged(
			resource.ID,
			snapshot.Configs,
			liveConfigs,
		) {
			result.Changed = append(
				result.Changed,
				resource,
			)
		}
	}

	sort.Slice(
		result.Missing,
		func(i, j int) bool {
			return result.Missing[i].ID <
				result.Missing[j].ID
		},
	)

	sort.Slice(
		result.Changed,
		func(i, j int) bool {
			return result.Changed[i].ID <
				result.Changed[j].ID
		},
	)

	return result
}

func configMap(
	configs []state.ResourceConfig,
) map[string][]state.ResourceConfig {

	result := make(
		map[string][]state.ResourceConfig,
	)

	for _, config := range configs {
		result[config.ResourceID] =
			append(
				result[config.ResourceID],
				config,
			)
	}

	return result
}

func configsChanged(
	resourceID string,
	snapshotConfigs []state.ResourceConfig,
	liveConfigs map[string][]state.ResourceConfig,
) bool {

	var expected []state.ResourceConfig

	for _, config := range snapshotConfigs {
		if config.ResourceID == resourceID {
			expected = append(expected, config)
		}
	}

	actual := liveConfigs[resourceID]

	if len(expected) != len(actual) {
		return true
	}

	normalize := func(
		input []state.ResourceConfig,
	) []string {

		result := make([]string, 0, len(input))

		for _, config := range input {

			data, err := json.Marshal(config.Properties)

			if err != nil {
				continue
			}

			result = append(
				result,
				string(data),
			)
		}

		sort.Strings(result)

		return result
	}

	return !reflect.DeepEqual(
		normalize(expected),
		normalize(actual),
	)
}
