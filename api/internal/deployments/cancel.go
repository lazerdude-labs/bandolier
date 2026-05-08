package deployments

import (
	"context"
	"errors"
)

// ErrNotCancellable is returned by Executor.Cancel when the named deployment
// has no live cancel func registered. This means either the deployment never
// started (race with the spawn site) or it already finished — either way the
// HTTP layer should respond 409 Conflict.
var ErrNotCancellable = errors.New("deployments: operation not cancellable")

// register stores the cancel func for a running deployment so the HTTP cancel
// handler can find it. Lazy-initialises cancelMap on first use to keep the
// Executor zero-value usable in tests that don't exercise cancellation.
func (e *Executor) register(depID string, cancel context.CancelFunc) {
	e.cancelMu.Lock()
	defer e.cancelMu.Unlock()
	if e.cancelMap == nil {
		e.cancelMap = make(map[string]context.CancelFunc)
	}
	e.cancelMap[depID] = cancel
}

// deregister removes the cancel func once the goroutine exits. Safe to call
// even if the deployment was already cancelled — Cancel itself only calls the
// func; the goroutine's defer is responsible for removing the entry regardless
// of how the work terminated.
func (e *Executor) deregister(depID string) {
	e.cancelMu.Lock()
	defer e.cancelMu.Unlock()
	delete(e.cancelMap, depID)
}

// Cancel signals the running goroutine for the named deployment to stop.
// Returns ErrNotCancellable when no live entry exists. The actual status
// transition (running → cancelled) happens inside the goroutine when its
// context is cancelled — Cancel itself does not write to the store.
func (e *Executor) Cancel(_ context.Context, depID string) error {
	e.cancelMu.Lock()
	cancel, ok := e.cancelMap[depID]
	e.cancelMu.Unlock()
	if !ok {
		return ErrNotCancellable
	}
	cancel()
	return nil
}
