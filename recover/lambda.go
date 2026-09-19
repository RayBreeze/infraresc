package recover

import (
	"context"
	"fmt"

	"infraresc/state"
)

func (e *Engine) recoverLambda(
	ctx context.Context,
	snapshot *state.Snapshot,
	item state.RecoveryItem,
) error {

	_ = ctx
	_ = snapshot

	return fmt.Errorf(
		"Lambda %s cannot be fully recovered: snapshot contains configuration but not deployable function code",
		item.ResourceID,
	)
}
