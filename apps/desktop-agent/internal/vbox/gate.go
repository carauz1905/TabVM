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
// same machine acquire the same mutex and therefore run one at a time, which
// prevents them from contending for the VirtualBox machine lock
// (VBOX_E_INVALID_OBJECT_STATE, "already has a lock request pending") and from
// flooding VBoxSVC. Commands that target different machines use different
// mutexes and still run in parallel.
type vmLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newVMLocker() *vmLocker {
	return &vmLocker{locks: make(map[string]*sync.Mutex)}
}

// mutexFor returns the mutex dedicated to id, creating it on first use. The
// per-id mutexes are never deleted: a host runs a small, bounded set of VMs, so
// the map stays tiny and keeping the entries avoids a race between deletion and
// a concurrent acquirer.
func (l *vmLocker) mutexFor(id string) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	m, ok := l.locks[id]
	if !ok {
		m = &sync.Mutex{}
		l.locks[id] = m
	}
	return m
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
// Locks are always acquired in the same order (the per-VM mutex first, then the
// global slot) and released in the reverse order, so the two layers can never
// deadlock. The global slot is the only place a caller blocks indefinitely, and
// that wait honours context cancellation (see exec); the per-VM mutex is only
// ever held for the duration of a single bounded RunContext, so it is never held
// while blocked forever.
//
// When vmID is empty the per-VM gate is skipped and only the global cap applies,
// which suits calls that create or address a VM before its UUID is known (e.g.
// appliance import, createvm --register, closemedium on an orphaned image).
func (s *service) runForVM(ctx context.Context, vmID, path string, args []string, timeout time.Duration) (runner.Result, error) {
	if vmID == "" {
		return s.exec(ctx, path, args, timeout)
	}

	mu := s.vmLocks.mutexFor(vmID)
	mu.Lock()
	defer mu.Unlock()

	return s.exec(ctx, path, args, timeout)
}
