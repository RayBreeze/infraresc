package recover

import (
	"context"
	"fmt"

	"infraresc/state"
)

func (e *Engine) recoverRDSInstance(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	_ = ctx
	_ = snapshot

	return fmt.Errorf(
		"RDS instance recovery for %s requires engine, storage, subnet/security configuration and restore/create parameters",
		item.ResourceID,
	)
}

func (e *Engine) recoverRDSCluster(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	_ = ctx
	_ = snapshot

	return fmt.Errorf(
		"RDS cluster recovery for %s requires a complete restore/create specification",
		item.ResourceID,
	)
}
