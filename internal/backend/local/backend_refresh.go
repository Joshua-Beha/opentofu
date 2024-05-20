// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package local

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/opentofu/opentofu/internal/backend"
	"github.com/opentofu/opentofu/internal/logging"
	"github.com/opentofu/opentofu/internal/states"
	"github.com/opentofu/opentofu/internal/states/statemgr"
	"github.com/opentofu/opentofu/internal/tfdiags"
)

func (b *Local) opRefresh(
	stopCtx context.Context,
	cancelCtx context.Context,
	op *backend.Operation,
	runningOp *backend.RunningOperation) {

	var diags tfdiags.Diagnostics

	// Check if our state exists if we're performing a refresh operation. We
	// only do this if we're managing state with this backend.
	if b.Backend == nil {
		if _, err := os.Stat(b.StatePath); err != nil {
			if os.IsNotExist(err) {
				err = nil
			}

			if err != nil {
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Cannot read state file",
					fmt.Sprintf("Failed to read %s: %s", b.StatePath, err),
				))
				op.ReportResult(runningOp, diags)
				return
			}
		}
	}

	// Refresh now happens via a plan, so we need to ensure this is enabled
	op.PlanRefresh = true

	// Get our context
	lr, _, opState, contextDiags := b.localRun(op)
	diags = diags.Append(contextDiags)
	if contextDiags.HasErrors() {
		op.ReportResult(runningOp, diags)
		return
	}

	// the state was locked during successful context creation; unlock the state
	// when the operation completes
	defer func() {
		diags := op.StateLocker.Unlock()
		if diags.HasErrors() {
			op.View.Diagnostics(diags)
			runningOp.Result = backend.OperationFailure
		}
	}()

	// If we succeed then we'll overwrite this with the resulting state below,
	// but otherwise the resulting state is just the input state.
	runningOp.State = lr.InputState.Mutable()
	if !runningOp.State.HasManagedResourceInstanceObjects() {
		diags = diags.Append(tfdiags.Sourceless(
			tfdiags.Warning,
			"Empty or non-existent state",
			"There are currently no remote objects tracked in the state, so there is nothing to refresh.",
		))
	}

	// get schemas before writing state
	schemas, moreDiags := lr.Core.Schemas(lr.Config, lr.InputState)
	diags = diags.Append(moreDiags)
	if moreDiags.HasErrors() {
		op.ReportResult(runningOp, diags)
		return
	}

	// Perform the refresh in a goroutine so we can be interrupted
	var newState states.ImmutableState
	var refreshDiags tfdiags.Diagnostics
	doneCh := make(chan struct{})
	panicHandler := logging.PanicHandlerWithTraceFn()
	go func() {
		defer panicHandler()
		defer close(doneCh)
		newState, refreshDiags = lr.Core.Refresh(lr.Config, lr.InputState, lr.PlanOpts)
		log.Printf("[INFO] backend/local: refresh calling Refresh")
	}()

	if b.opWait(doneCh, stopCtx, cancelCtx, lr.Core, opState, op.View) {
		return
	}

	// Write the resulting state to the running op
	runningOp.State = newState.Mutable()
	diags = diags.Append(refreshDiags)
	if refreshDiags.HasErrors() {
		op.ReportResult(runningOp, diags)
		return
	}

	err := statemgr.WriteAndPersist(opState, newState, schemas)
	if err != nil {
		diags = diags.Append(fmt.Errorf("failed to write state: %w", err))
		op.ReportResult(runningOp, diags)
		return
	}

	// Show any remaining warnings before exiting
	op.ReportResult(runningOp, diags)
}
