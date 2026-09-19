package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	infraAWS "infraresc/aws"
	infraCrypto "infraresc/crypto"
	infraMedia "infraresc/media"
	infraRuntime "infraresc/runtime"
	"infraresc/state"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare a snapshot from physical media with live AWS infrastructure",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		fmt.Println("InfraResc Infrastructure Diff")
		fmt.Println()

		snapshotPath, err := chooseDiffSnapshot()
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("Selected snapshot: %s\n", snapshotPath)

		fmt.Println()
		fmt.Print("Encryption password: ")

		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()

		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}

		password := strings.TrimSpace(string(passwordBytes))
		passwordBytes = nil

		if password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		fmt.Println()
		fmt.Println("Decrypting snapshot...")

		snapshot, err := loadDiffSnapshot(
			snapshotPath,
			password,
		)

		password = ""

		if err != nil {
			return err
		}

		fmt.Println("Authentication: OK")

		if err := validateDiffSnapshot(snapshot); err != nil {
			return fmt.Errorf(
				"snapshot validation failed: %w",
				err,
			)
		}

		fmt.Println("Snapshot validation: OK")

		fmt.Println()
		fmt.Println("Connecting to live AWS infrastructure...")

		rt, err := infraRuntime.Initialize(ctx, "")
		if err != nil {
			return fmt.Errorf(
				"initializing AWS runtime: %w",
				err,
			)
		}

		fmt.Printf(
			"Account: %s\n",
			rt.Identity.AccountID,
		)

		fmt.Printf(
			"Region:  %s\n",
			rt.Identity.Region,
		)

		if snapshot.AccountID != rt.Identity.AccountID {
			return fmt.Errorf(
				"snapshot account %s does not match live AWS account %s",
				snapshot.AccountID,
				rt.Identity.AccountID,
			)
		}

		if snapshot.Region != rt.Identity.Region {
			return fmt.Errorf(
				"snapshot region %s does not match live AWS region %s",
				snapshot.Region,
				rt.Identity.Region,
			)
		}

		fmt.Println()
		fmt.Println("Discovering live AWS infrastructure...")

		collector := infraAWS.NewCollector(rt.AWS)

		liveInfrastructure, err := collector.Collect(ctx)
		if err != nil {
			return fmt.Errorf(
				"discovering live infrastructure: %w",
				err,
			)
		}

		fmt.Printf(
			"  Live resources:     %d\n",
			len(liveInfrastructure.Resources),
		)

		fmt.Printf(
			"  Live relationships: %d\n",
			len(liveInfrastructure.Edges),
		)

		fmt.Println()
		fmt.Println("Capturing live configuration...")

		liveSnapshotCollector :=
			infraAWS.NewSnapshotCollector(rt.AWS)

		liveSnapshot, err := liveSnapshotCollector.Collect(
			ctx,
			rt.Identity.AccountID,
			rt.Identity.Region,
			liveInfrastructure.Resources,
			liveInfrastructure.Edges,
		)

		if err != nil {
			return fmt.Errorf(
				"capturing live configuration: %w",
				err,
			)
		}

		fmt.Printf(
			"  Live configurations: %d\n",
			len(liveSnapshot.Configs),
		)

		fmt.Println()
		fmt.Println("Comparing snapshot with live infrastructure...")

		result := diffInfrastructure(
			snapshot,
			liveSnapshot,
		)

		printDiffResult(result)

		return nil
	},
}

type infrastructureDiff struct {
	AddedResources   []state.Resource
	RemovedResources []state.Resource

	AddedEdges   []state.Edge
	RemovedEdges []state.Edge

	ModifiedConfigs []configChange

	UnchangedResources int
	UnchangedConfigs   int

	SnapshotWarnings []state.SnapshotWarning
	LiveWarnings     []state.SnapshotWarning
}

type configChange struct {
	ResourceID string
	Type       string
	Changes    []propertyChange
}

type propertyChange struct {
	Path     string
	Snapshot interface{}
	Live     interface{}
}

func loadDiffSnapshot(
	path string,
	password string,
) (*state.Snapshot, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"reading snapshot: %w",
			err,
		)
	}

	plaintext, err := infraCrypto.Open(
		data,
		password,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"decrypting snapshot: %w",
			err,
		)
	}

	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	var snapshot state.Snapshot

	if err := json.Unmarshal(
		plaintext,
		&snapshot,
	); err != nil {
		return nil, fmt.Errorf(
			"decoding snapshot: %w",
			err,
		)
	}

	return &snapshot, nil
}

func validateDiffSnapshot(
	snapshot *state.Snapshot,
) error {

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if snapshot.Version == "" {
		return fmt.Errorf("snapshot version is missing")
	}

	if snapshot.AccountID == "" {
		return fmt.Errorf("snapshot account ID is missing")
	}

	if snapshot.Region == "" {
		return fmt.Errorf("snapshot region is missing")
	}

	resourceIDs := make(
		map[string]struct{},
		len(snapshot.Resources),
	)

	for _, resource := range snapshot.Resources {

		if resource.ID == "" {
			return fmt.Errorf(
				"resource has empty ID",
			)
		}

		if resource.ARN == "" {
			return fmt.Errorf(
				"resource %s has empty ARN",
				resource.ID,
			)
		}

		if resource.ID != resource.ARN {
			return fmt.Errorf(
				"resource identity mismatch: ID=%s ARN=%s",
				resource.ID,
				resource.ARN,
			)
		}

		if _, exists := resourceIDs[resource.ID]; exists {
			return fmt.Errorf(
				"duplicate resource ID: %s",
				resource.ID,
			)
		}

		resourceIDs[resource.ID] = struct{}{}
	}

	for _, edge := range snapshot.Edges {

		if edge.From == "" || edge.To == "" {
			return fmt.Errorf(
				"relationship contains empty endpoint",
			)
		}

		if edge.Relation == "" {
			return fmt.Errorf(
				"relationship has empty relation",
			)
		}

		if _, exists := resourceIDs[edge.From]; !exists {
			return fmt.Errorf(
				"relationship source does not exist: %s",
				edge.From,
			)
		}

		if _, exists := resourceIDs[edge.To]; !exists {
			return fmt.Errorf(
				"relationship target does not exist: %s",
				edge.To,
			)
		}
	}

	for _, config := range snapshot.Configs {

		if config.ResourceID == "" {
			return fmt.Errorf(
				"configuration has empty resource ID",
			)
		}

		if _, exists := resourceIDs[config.ResourceID]; !exists {
			return fmt.Errorf(
				"configuration references unknown resource: %s",
				config.ResourceID,
			)
		}
	}

	return nil
}

func diffInfrastructure(
	snapshot *state.Snapshot,
	live *state.Snapshot,
) infrastructureDiff {

	result := infrastructureDiff{
		SnapshotWarnings: snapshot.Warnings,
		LiveWarnings:     live.Warnings,
	}

	snapshotResources := make(
		map[string]state.Resource,
		len(snapshot.Resources),
	)

	liveResources := make(
		map[string]state.Resource,
		len(live.Resources),
	)

	for _, resource := range snapshot.Resources {
		snapshotResources[resource.ID] = resource
	}

	for _, resource := range live.Resources {
		liveResources[resource.ID] = resource
	}

	for id, resource := range liveResources {
		if _, exists := snapshotResources[id]; !exists {
			result.AddedResources = append(
				result.AddedResources,
				resource,
			)
		}
	}

	for id, resource := range snapshotResources {
		if _, exists := liveResources[id]; !exists {
			result.RemovedResources = append(
				result.RemovedResources,
				resource,
			)
		}
	}

	result.UnchangedResources =
		len(snapshotResources) -
			len(result.RemovedResources)

	snapshotEdges := edgeSet(snapshot.Edges)
	liveEdges := edgeSet(live.Edges)

	for key, edge := range liveEdges {
		if _, exists := snapshotEdges[key]; !exists {
			result.AddedEdges = append(
				result.AddedEdges,
				edge,
			)
		}
	}

	for key, edge := range snapshotEdges {
		if _, exists := liveEdges[key]; !exists {
			result.RemovedEdges = append(
				result.RemovedEdges,
				edge,
			)
		}
	}

	snapshotConfigs := configMap(snapshot.Configs)
	liveConfigs := configMap(live.Configs)

	for id, liveConfig := range liveConfigs {

		snapshotConfig, exists := snapshotConfigs[id]
		if !exists {
			continue
		}

		changes := diffProperties(
			"",
			snapshotConfig.Properties,
			liveConfig.Properties,
		)

		if len(changes) == 0 {
			result.UnchangedConfigs++
			continue
		}

		result.ModifiedConfigs = append(
			result.ModifiedConfigs,
			configChange{
				ResourceID: id,
				Type:       liveConfig.Type,
				Changes:    changes,
			},
		)
	}

	return result
}

func edgeSet(
	edges []state.Edge,
) map[string]state.Edge {

	result := make(
		map[string]state.Edge,
		len(edges),
	)

	for _, edge := range edges {

		key :=
			edge.From +
				"\x00" +
				edge.To +
				"\x00" +
				edge.Relation

		result[key] = edge
	}

	return result
}

func configMap(
	configs []state.ResourceConfig,
) map[string]state.ResourceConfig {

	result := make(
		map[string]state.ResourceConfig,
		len(configs),
	)

	for _, config := range configs {
		result[config.ResourceID] = config
	}

	return result
}

func diffProperties(
	path string,
	snapshot interface{},
	live interface{},
) []propertyChange {

	if reflect.DeepEqual(snapshot, live) {
		return nil
	}

	switch snapshotValue := snapshot.(type) {

	case map[string]interface{}:

		liveValue, ok := live.(map[string]interface{})

		if !ok {
			return []propertyChange{
				{
					Path:     path,
					Snapshot: snapshot,
					Live:     live,
				},
			}
		}

		keys := make(
			map[string]struct{},
			len(snapshotValue)+len(liveValue),
		)

		for key := range snapshotValue {
			keys[key] = struct{}{}
		}

		for key := range liveValue {
			keys[key] = struct{}{}
		}

		sortedKeys := make(
			[]string,
			0,
			len(keys),
		)

		for key := range keys {
			sortedKeys = append(
				sortedKeys,
				key,
			)
		}

		sort.Strings(sortedKeys)

		var changes []propertyChange

		for _, key := range sortedKeys {

			childPath := key

			if path != "" {
				childPath = path + "." + key
			}

			var snapshotChild interface{}
			var liveChild interface{}

			snapshotChild = snapshotValue[key]
			liveChild = liveValue[key]

			changes = append(
				changes,
				diffProperties(
					childPath,
					snapshotChild,
					liveChild,
				)...,
			)
		}

		return changes

	default:

		return []propertyChange{
			{
				Path:     path,
				Snapshot: snapshot,
				Live:     live,
			},
		}
	}
}

func printDiffResult(
	result infrastructureDiff,
) {

	fmt.Println()
	fmt.Println("Infrastructure Diff")
	fmt.Println("========================================")

	fmt.Println()
	fmt.Println("Resources")
	fmt.Println("----------------------------------------")

	fmt.Printf(
		"Added to live:      %d\n",
		len(result.AddedResources),
	)

	for _, resource := range sortedResources(
		result.AddedResources,
	) {
		fmt.Printf(
			"  + %s [%s]\n",
			resource.ARN,
			resource.Type,
		)
	}

	fmt.Printf(
		"Missing from live:  %d\n",
		len(result.RemovedResources),
	)

	for _, resource := range sortedResources(
		result.RemovedResources,
	) {
		fmt.Printf(
			"  - %s [%s]\n",
			resource.ARN,
			resource.Type,
		)
	}

	fmt.Printf(
		"Unchanged:          %d\n",
		result.UnchangedResources,
	)

	fmt.Println()
	fmt.Println("Relationships")
	fmt.Println("----------------------------------------")

	fmt.Printf(
		"Added to live:      %d\n",
		len(result.AddedEdges),
	)

	for _, edge := range sortedEdges(
		result.AddedEdges,
	) {
		fmt.Printf(
			"  + %s --[%s]--> %s\n",
			edge.From,
			edge.Relation,
			edge.To,
		)
	}

	fmt.Printf(
		"Missing from live:  %d\n",
		len(result.RemovedEdges),
	)

	for _, edge := range sortedEdges(
		result.RemovedEdges,
	) {
		fmt.Printf(
			"  - %s --[%s]--> %s\n",
			edge.From,
			edge.Relation,
			edge.To,
		)
	}

	fmt.Println()
	fmt.Println("Configuration Changes")
	fmt.Println("----------------------------------------")

	fmt.Printf(
		"Modified resources: %d\n",
		len(result.ModifiedConfigs),
	)

	for _, change := range sortedConfigChanges(
		result.ModifiedConfigs,
	) {

		fmt.Println()
		fmt.Printf(
			"  %s [%s]\n",
			change.ResourceID,
			change.Type,
		)

		for _, property := range change.Changes {

			fmt.Printf(
				"    %s\n",
				property.Path,
			)

			fmt.Printf(
				"      snapshot: %v\n",
				property.Snapshot,
			)

			fmt.Printf(
				"      live:     %v\n",
				property.Live,
			)
		}
	}

	fmt.Printf(
		"Unchanged configs:  %d\n",
		result.UnchangedConfigs,
	)

	if len(result.SnapshotWarnings) > 0 ||
		len(result.LiveWarnings) > 0 {

		fmt.Println()
		fmt.Println("Warnings")
		fmt.Println("----------------------------------------")

		for _, warning := range result.SnapshotWarnings {
			fmt.Printf(
				"  snapshot: %s: %s\n",
				warning.ResourceID,
				warning.Message,
			)
		}

		for _, warning := range result.LiveWarnings {
			fmt.Printf(
				"  live: %s: %s\n",
				warning.ResourceID,
				warning.Message,
			)
		}
	}

	fmt.Println()
	fmt.Println("Summary")
	fmt.Println("----------------------------------------")

	totalChanges :=
		len(result.AddedResources) +
			len(result.RemovedResources) +
			len(result.AddedEdges) +
			len(result.RemovedEdges) +
			len(result.ModifiedConfigs)

	if totalChanges == 0 {
		fmt.Println("No differences detected.")
	} else {
		fmt.Printf(
			"Total detected changes: %d\n",
			totalChanges,
		)
	}
}

func sortedResources(
	resources []state.Resource,
) []state.Resource {

	result := append(
		[]state.Resource(nil),
		resources...,
	)

	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].ARN < result[j].ARN
		},
	)

	return result
}

func sortedEdges(
	edges []state.Edge,
) []state.Edge {

	result := append(
		[]state.Edge(nil),
		edges...,
	)

	sort.Slice(
		result,
		func(i, j int) bool {

			left :=
				result[i].From +
					result[i].Relation +
					result[i].To

			right :=
				result[j].From +
					result[j].Relation +
					result[j].To

			return left < right
		},
	)

	return result
}

func sortedConfigChanges(
	changes []configChange,
) []configChange {

	result := append(
		[]configChange(nil),
		changes...,
	)

	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].ResourceID <
				result[j].ResourceID
		},
	)

	return result
}

// chooseDiffSnapshot searches physical media registered by the
// InfraResc media subsystem, then lets the user select a snapshot.
func chooseDiffSnapshot() (string, error) {

	fmt.Println("Searching for InfraResc physical media...")
	fmt.Println()

	mediaRoots, err := infraMedia.DiscoverExternalMedia()
	if err != nil {
		return "", fmt.Errorf(
			"discovering external media: %w",
			err,
		)
	}

	var repositories []string

	for _, root := range mediaRoots {

		if infraMedia.IsInfraRescMedia(root) {
			repositories = append(
				repositories,
				root,
			)
		}
	}

	if len(repositories) == 0 {
		return "", fmt.Errorf(
			"no InfraResc physical media detected",
		)
	}

	fmt.Println("InfraResc media:")

	for i, root := range repositories {

		manifest, err := infraMedia.LoadManifest(root)
		if err != nil {
			fmt.Printf(
				"%d. %s (manifest error)\n",
				i+1,
				root,
			)
			continue
		}

		fmt.Printf(
			"%d. %s (%d snapshots)\n",
			i+1,
			root,
			len(manifest.Snapshots),
		)
	}

	fmt.Println()
	fmt.Print("Select media: ")

	var selection string

	if _, err := fmt.Scanln(&selection); err != nil {
		return "", fmt.Errorf(
			"reading media selection: %w",
			err,
		)
	}

	var mediaIndex int

	if _, err := fmt.Sscanf(
		selection,
		"%d",
		&mediaIndex,
	); err != nil {
		return "", fmt.Errorf(
			"invalid media selection",
		)
	}

	if mediaIndex < 1 ||
		mediaIndex > len(repositories) {
		return "", fmt.Errorf(
			"media selection out of range",
		)
	}

	mediaRoot := repositories[mediaIndex-1]

	manifest, err := infraMedia.LoadManifest(
		mediaRoot,
	)
	if err != nil {
		return "", fmt.Errorf(
			"loading media manifest: %w",
			err,
		)
	}

	if len(manifest.Snapshots) == 0 {
		return "", fmt.Errorf(
			"selected media contains no snapshots",
		)
	}

	fmt.Println()
	fmt.Printf(
		"Snapshots on %s:\n",
		mediaRoot,
	)
	fmt.Println()

	for i, snapshot := range manifest.Snapshots {

		fmt.Printf(
			"%d. %s (%s)\n",
			i+1,
			snapshot.Filename,
			formatBytes(snapshot.Size),
		)
	}

	fmt.Println()
	fmt.Print("Select snapshot: ")

	if _, err := fmt.Scanln(&selection); err != nil {
		return "", fmt.Errorf(
			"reading snapshot selection: %w",
			err,
		)
	}

	var snapshotIndex int

	if _, err := fmt.Sscanf(
		selection,
		"%d",
		&snapshotIndex,
	); err != nil {
		return "", fmt.Errorf(
			"invalid snapshot selection",
		)
	}

	if snapshotIndex < 1 ||
		snapshotIndex > len(manifest.Snapshots) {
		return "", fmt.Errorf(
			"snapshot selection out of range",
		)
	}

	snapshot := manifest.Snapshots[snapshotIndex-1]

	return filepath.Join(
		infraMedia.SnapshotPath(mediaRoot),
		snapshot.Filename,
	), nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
