// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package operation

import (
	"fmt"

	"github.com/juju/errors"

	"github.com/juju/juju/worker/uniter/runner"
)

type runAction struct {
	actionId string

	callbacks     Callbacks
	runnerFactory runner.Factory

	name   string
	runner runner.Runner
}

// String is part of the Operation interface.
func (ra *runAction) String() string {
	return fmt.Sprintf("run action %s", ra.actionId)
}

// Prepare ensures that the action is valid and can be executed. If not, it
// will return ErrSkipExecute. It preserves any hook recorded in the supplied
// state.
// Prepare is part of the Operation interface.
func (ra *runAction) Prepare(state State) (*State, error) {
	rnr, err := ra.runnerFactory.NewActionRunner(ra.actionId)
	if cause := errors.Cause(err); runner.IsBadActionError(cause) {
		if err := ra.callbacks.FailAction(ra.actionId, err.Error()); err != nil {
			return nil, err
		}
		return nil, ErrSkipExecute
	} else if cause == runner.ErrActionNotAvailable {
		return nil, ErrSkipExecute
	} else if err != nil {
		return nil, errors.Annotatef(err, "cannot create runner for action %q", ra.actionId)
	}
	actionData, err := rnr.Context().ActionData()
	if err != nil {
		// this should *really* never happen, but let's not panic
		return nil, errors.Trace(err)
	}
	ra.name = actionData.ActionName
	ra.runner = rnr
	return stateChange{
		Kind:     RunAction,
		Step:     Pending,
		ActionId: &ra.actionId,
		Hook:     state.Hook,
	}.apply(state), nil
}

// Execute runs the action, and preserves any hook recorded in the supplied state.
// Execute is part of the Operation interface.
func (ra *runAction) Execute(state State) (*State, error) {
	message := fmt.Sprintf("running action %s", ra.name)
	unlock, err := ra.callbacks.AcquireExecutionLock(message)
	if err != nil {
		return nil, err
	}
	defer unlock()

	err = ra.runner.RunAction(ra.name)
	if err != nil {
		// This indicates an actual error -- an action merely failing should
		// be handled inside the Runner, and returned as nil.
		return nil, errors.Annotatef(err, "running action %q", ra.name)
	}
	return stateChange{
		Kind:     RunAction,
		Step:     Done,
		ActionId: &ra.actionId,
		Hook:     state.Hook,
	}.apply(state), nil
}

// Commit preserves the recorded hook, and returns a neutral state.
// Commit is part of the Operation interface.
func (ra *runAction) Commit(state State) (*State, error) {
	kind := Continue
	if state.Hook != nil {
		kind = RunHook
	}
	return stateChange{
		Kind: kind,
		Step: Pending,
		Hook: state.Hook,
	}.apply(state), nil
}
