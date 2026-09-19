package recover

import (
	"context"
	"fmt"

	"infraresc/state"
)

func (e *Engine) recoverDynamoDB(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	_ = ctx
	_ = snapshot

	return fmt.Errorf(
		"DynamoDB table recovery for %s requires a complete CreateTable specification",
		item.ResourceID,
	)
}
