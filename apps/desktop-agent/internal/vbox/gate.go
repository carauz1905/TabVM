package vbox

import (
	"context"
	"sync"
	"time"

	"github.com/tabvm/desktop-agent/internal/runner"
)

// maxConcurrentVBox bounds how many VBoxManage processes the agent runs at once
// across all VMs. VBoxSVC serializes much of its own work internally, and a
// burst of dozens of concurrent processes can wedge it entirely, so this is a
// global safety net layered on top of the per-VM serialization gate. Different
// VMs still run in parallel up to this cap.
const maxConcurrentVBox = 4

// vboxSlots is the package-level global concurrency cap shared by every
// VBoxManage invocation issued through the service. It is a buffered channel
// used as a counting semaphore: acquiring a slot sends into it, releasing
// receives from it.
var vboxSlots = make(chan struct{}, maxConcurrentVBox)

// vmLocker serializes VBoxManage executions per VM id. Commands that target the
// same machine acquire the same 1-slot channel semaphore and therefore run one
// at a time, which prevents them from contending for the VirtualBox machine lock
// (VBOX_E_INVALID_OBJECT_STATE, "already has a lock request pending") and from
// flooding VBoxSVC. Commands that target different machines use different
// channels and still run in parallel. A channel (not a sync.Mutex) is used so
// acquisition can be selected against ctx.Done() — see runForVM.
type vmLocker struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func newVMLocker() *vmLocker {
	return &vmLocker{locks: make(map[string]chan struct{})}
}

// chanFor returns the 1-slot semaphore channel dedicated to id, creating it on
// first use. The per-id channels are never deleted: a host runs a small, bounded
// set of VMs, so the map stays tiny and keeping the entries avoids a race
// between deletion and a concurrent acquirer.
func (l *vmLocker) chanFor(id string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.locks[id]
	if !ok {
		c = make(chan struct{}, 1)
		l.locks[id] = c
	}
	return c
}

// exec runs VBoxManage under the global concurrency cap only. It is the single
// chokepoint every VBoxManage invocation in this package flows through, so the
// cap is enforced uniformly. Use it directly for calls that do not target a
// specific VM (host discovery, `list vms`, `--version`); VM-targeting calls go
// through runForVM, which layers the per-VM gate on top. It respects context
// cancellation while waiting for a free slot so a cancelled request never blocks
// indefinitely behind a saturated cap.
func (s *service) exec(ctx context.Context, path string, args []string, timeout time.Duration) (runner.Result, error) {
	select {
	case vboxSlots <- struct{}{}:
	case <-ctx.Done():
		return runner.Result{ExitCode: -1}, ctx.Err()
	}
	defer func() { <-vboxSlots }()

	return s.runner.RunContext(ctx, path, args, timeout)
}

// runForVM runs a VBoxManage command that targets a specific VM. It serializes
// execution per VM id — so the focus-open burst on a single VM no longer
// contends for the machine lock — and then applies the global concurrency cap.
//
// Both waits (the per-VM slot and, inside exec, the global slot) honour context
// cancellation: a request that gives up while the VM is busy — e.g. blocked
// behind a multi-minute export/clone that holds the per-VM slot — fails fast
// with ctx.Err() instead of leaking a goroutine that hangs until the previous
// command finishes. Slots are acquired per-VM-then-global and released in the
// reverse order, so the two layers can never deadlock, and the per-VM slot is
// only ever held for a single bounded RunContext.
//
// Bulk iterations that read many VMs (e.g. enhanceVmStates) must NOT use this —
// they call s.exec directly so a single busy VM cannot stall the whole loop.
//
// When vmID is empty the per-VM gate is skipped and only the global cap applies,
// which suits calls that create or address a VM before its UUID is known (e.g.
// appliance import, createvm --register, closemedium on an orphaned image).
func (s *service) runForVM(ctx context.Context, vmID, path string, args []string, timeout time.Duration) (runner.Result, error) {
	if vmID == "" {
		return s.exec(ctx, path, args, timeout)
	}

	ch := s.vmLocks.chanFor(vmID)
	select {
	case ch <- struct{}{}:
	case <-ctx.Done():
		return runner.Result{ExitCode: -1}, ctx.Err()
	}
	defer func() { <-ch }()

	return s.exec(ctx, path, args, timeout)
}
