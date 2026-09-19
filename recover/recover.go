package recover

import (
	"context"
	"fmt"

	"infraresc/aws"
	"infraresc/state"
)

type Engine struct {
	AWS *aws.Client
}

func New(client *aws.Client) *Engine {
	return &Engine{
		AWS: client,
	}
}

func (e *Engine) Run(
	ctx context.Context,
	snapshot *state.Snapshot,
) (*state.RecoveryResult, error) {

	if snapshot == nil {
		return nil, fmt.Errorf("snapshot is nil")
	}

	if e.AWS == nil {
		return nil, fmt.Errorf("AWS client is nil")
	}

	/*
	 * Discover current infrastructure.
	 */
	collector := aws.NewCollector(e.AWS)

	liveInfrastructure, err := collector.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"discover live infrastructure: %w",
			err,
		)
	}

	/*
	 * Capture the current configuration so that
	 * changed resources can be detected.
	 */
	snapshotCollector := aws.NewSnapshotCollector(e.AWS)

	liveSnapshot, err := snapshotCollector.Collect(
		ctx,
		snapshot.AccountID,
		snapshot.Region,
		liveInfrastructure.Resources,
		liveInfrastructure.Edges,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"capture live infrastructure: %w",
			err,
		)
	}

	/*
	 * First verification.
	 */
	changes := Compare(
		snapshot,
		liveSnapshot,
	)

	items := make(
		map[string]state.RecoveryItem,
	)

	for _, resource := range changes.Missing {
		items[resource.ID] = state.RecoveryItem{
			ResourceID:   resource.ID,
			ResourceType: resource.Type,
			Action:       state.RecoveryMissing,
		}
	}

	for _, resource := range changes.Changed {
		items[resource.ID] = state.RecoveryItem{
			ResourceID:   resource.ID,
			ResourceType: resource.Type,
			Action:       state.RecoveryChanged,
		}
	}

	result := &state.RecoveryResult{
		Items: make(
			[]state.RecoveryItem,
			0,
			len(items),
		),
		RecoveredIDs: make(
			[]string,
			0,
			len(items),
		),
	}

	for _, item := range items {
		result.Items = append(
			result.Items,
			item,
		)
	}

	/*
	 * Nothing requires recovery.
	 */
	if len(items) == 0 {
		result.Skipped = len(snapshot.Resources)

		return result, nil
	}

	/*
	 * Resolve dependency order.
	 */
	order, err := RecoveryOrder(
		snapshot,
		items,
	)

	if err != nil {
		return nil, err
	}

	/*
	 * Recover only resources that actually need recovery.
	 */
	for _, id := range order {

		item := items[id]

		result.Attempted++

		if err := e.recoverResource(
			ctx,
			snapshot,
			item,
		); err != nil {

			result.Failed++

			result.Errors = append(
				result.Errors,
				fmt.Sprintf(
					"%s: %v",
					item.ResourceID,
					err,
				),
			)

			continue
		}

		/*
		 * Track successful recovery explicitly.
		 *
		 * This is important because resources that failed
		 * recovery must not make the entire operation look
		 * like a verification failure later.
		 */
		result.Recovered++

		result.RecoveredIDs = append(
			result.RecoveredIDs,
			item.ResourceID,
		)
	}

	/*
	 * Second verification.
	 *
	 * Re-discover the infrastructure after recovery.
	 */
	finalInfrastructure, err := collector.Collect(ctx)
	if err != nil {
		return result, fmt.Errorf(
			"post-recovery discovery: %w",
			err,
		)
	}

	finalSnapshot, err := snapshotCollector.Collect(
		ctx,
		snapshot.AccountID,
		snapshot.Region,
		finalInfrastructure.Resources,
		finalInfrastructure.Edges,
	)

	if err != nil {
		return result, fmt.Errorf(
			"post-recovery snapshot: %w",
			err,
		)
	}

	finalChanges := Compare(
		snapshot,
		finalSnapshot,
	)

	/*
	 * Only verify resources that InfraResc actually
	 * recovered successfully.
	 *
	 * Example:
	 *
	 *   S3 bucket       -> recovered successfully
	 *   Lambda A        -> recovery failed
	 *   Lambda B        -> recovery failed
	 *
	 * The final verification must verify the S3 bucket,
	 * but must not turn Lambda A/B failures into another
	 * global verification failure.
	 */
	recoveredSet := make(
		map[string]struct{},
		len(result.RecoveredIDs),
	)

	for _, id := range result.RecoveredIDs {
		recoveredSet[id] = struct{}{}
	}

	verificationMissing := 0
	verificationChanged := 0

	for _, resource := range finalChanges.Missing {
		if _, ok := recoveredSet[resource.ID]; ok {
			verificationMissing++
		}
	}

	for _, resource := range finalChanges.Changed {
		if _, ok := recoveredSet[resource.ID]; ok {
			verificationChanged++
		}
	}

	/*
	 * A resource that InfraResc successfully recovered but
	 * still doesn't exist or still differs is a genuine
	 * recovery verification failure.
	 */
	if verificationMissing > 0 ||
		verificationChanged > 0 {

		if result.Failed == 0 {
			return result, fmt.Errorf(
				"recovery verification failed: %d recovered resources missing, %d recovered resources changed",
				verificationMissing,
				verificationChanged,
			)
		}

		result.Errors = append(
			result.Errors,
			fmt.Sprintf(
				"post-recovery verification: %d recovered resources missing, %d recovered resources changed",
				verificationMissing,
				verificationChanged,
			),
		)
	}

	/*
	 * Anything that was not attempted and did not require
	 * recovery is considered skipped.
	 *
	 * Failed resources remain represented by result.Failed.
	 */
	result.Skipped =
		len(snapshot.Resources) -
			result.Attempted

	if result.Skipped < 0 {
		result.Skipped = 0
	}

	return result, nil
}
