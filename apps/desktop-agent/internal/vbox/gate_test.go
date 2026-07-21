package vbox

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/runner"
)

// countingRunner records how many RunContext calls are in flight at once, both
// overall and per command key, and can either hold each call open for a fixed
// duration (hold) or block it on a channel (release) so a test can observe real
// overlap. It is the instrument the serialization-gate tests use to prove that
// same-VM calls never overlap, different-VM calls can, and the global cap is
// never exceeded.
type countingRunner struct {
	mu        sync.Mutex
	active    int
	maxActive int
	perKey    map[string]int
	perKeyMax map[string]int

	entered chan string   // if non-nil, receives each call's key when it starts
	release chan struct{} // if non-nil, each call blocks on it before returning
	hold    time.Duration // if release is nil, each call sleeps this long
}

func newCountingRunner() *countingRunner {
	return &countingRunner{perKey: map[string]int{}, perKeyMax: map[string]int{}}
}

func (r *countingRunner) RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (runner.Result, error) {
	key := joinArgs(args)

	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.perKey[key]++
	if r.perKey[key] > r.perKeyMax[key] {
		r.perKeyMax[key] = r.perKey[key]
	}
	r.mu.Unlock()

	if r.entered != nil {
		r.entered <- key
	}
	if r.release != nil {
		<-r.release
	} else if r.hold > 0 {
		time.Sleep(r.hold)
	}

	r.mu.Lock()
	r.active--
	r.perKey[key]--
	r.mu.Unlock()

	return runner.Result{ExitCode: 0}, nil
}

func (r *countingRunner) maxConcurrency() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxActive
}

func (r *countingRunner) maxConcurrencyForKey(key string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.perKeyMax[key]
}

// TestRunForVM_SerializesSameVM proves that concurrent VBoxManage calls
// targeting the same VM never run at the same time, so the focus-open burst can
// no longer contend for the machine lock.
func TestRunForVM_SerializesSameVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox serialization tests run on Windows in CI")
	}

	run := newCountingRunner()
	run.hold = 40 * time.Millisecond
	svc := &service{runner: run, vmLocks: newVMLocker()}

	id := "11111111-1111-1111-1111-111111111111"
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.runForVM(context.Background(), id, "VBoxManage", []string{"showvminfo", id}, time.Second)
		}()
	}
	wg.Wait()

	if got := run.maxConcurrency(); got != 1 {
		t.Fatalf("expected same-VM calls to be serialized (max concurrency 1), got %d", got)
	}
}

// TestRunForVM_ParallelDifferentVMs proves that calls targeting different VMs
// still run in parallel: the per-VM gate must not become a global bottleneck.
func TestRunForVM_ParallelDifferentVMs(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox serialization tests run on Windows in CI")
	}

	run := newCountingRunner()
	run.hold = 40 * time.Millisecond
	svc := &service{runner: run, vmLocks: newVMLocker()}

	ids := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.runForVM(context.Background(), id, "VBoxManage", []string{"showvminfo", id}, time.Second)
		}()
	}
	wg.Wait()

	if got := run.maxConcurrency(); got < 2 {
		t.Fatalf("expected different-VM calls to run in parallel (max concurrency >= 2), got %d", got)
	}
}

// TestRunForVM_GlobalCapLimitsConcurrency proves the global safety net: even
// across many different VMs, the agent never runs more than maxConcurrentVBox
// VBoxManage processes at once, so a burst can never flood VBoxSVC.
func TestRunForVM_GlobalCapLimitsConcurrency(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox serialization tests run on Windows in CI")
	}

	run := newCountingRunner()
	run.entered = make(chan string, 64)
	run.release = make(chan struct{})
	svc := &service{runner: run, vmLocks: newVMLocker()}

	const goroutines = maxConcurrentVBox * 3
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		id := fmt.Sprintf("vm-%02d", i) // distinct id per goroutine, so per-VM gates never serialize them
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.runForVM(context.Background(), id, "VBoxManage", []string{"showvminfo", id}, time.Second)
		}()
	}

	// Exactly maxConcurrentVBox calls should reach RunContext; the rest block on
	// the global semaphore before starting a process.
	for i := 0; i < maxConcurrentVBox; i++ {
		select {
		case <-run.entered:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected %d concurrent calls to start, only %d did", maxConcurrentVBox, i)
		}
	}
	select {
	case <-run.entered:
		t.Fatalf("a call started beyond the global cap of %d", maxConcurrentVBox)
	case <-time.After(150 * time.Millisecond):
	}

	close(run.release)
	wg.Wait()

	if got := run.maxConcurrency(); got != maxConcurrentVBox {
		t.Fatalf("expected max concurrency to equal the cap %d, got %d", maxConcurrentVBox, got)
	}
}

// TestReadShowVmInfo_SerializesSameVM proves the gate reaches a real read helper:
// two concurrent readShowVmInfo calls for the same VM are serialized end to end.
func TestReadShowVmInfo_SerializesSameVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox serialization tests run on Windows in CI")
	}

	run := newCountingRunner()
	run.hold = 40 * time.Millisecond
	svc := &service{runner: run, vmLocks: newVMLocker()}

	id := "11111111-1111-1111-1111-111111111111"
	key := joinArgs(showVmInfoArgs(id))

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.readShowVmInfo(context.Background(), "VBoxManage", id, "reading VM state")
		}()
	}
	wg.Wait()

	if got := run.maxConcurrencyForKey(key); got != 1 {
		t.Fatalf("expected concurrent readShowVmInfo for the same VM to be serialized, got max concurrency %d", got)
	}
}

// TestExec_AppliesGlobalCapWithoutPerVMGate proves non-VM calls (list vms,
// --version, host discovery) still ride the global cap but are not serialized
// per VM, so they never queue behind an unrelated VM's commands.
func TestExec_AppliesGlobalCapWithoutPerVMGate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox serialization tests run on Windows in CI")
	}

	run := newCountingRunner()
	run.hold = 30 * time.Millisecond
	svc := &service{runner: run, vmLocks: newVMLocker()}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.exec(context.Background(), "VBoxManage", listVmsArgs(), time.Second)
		}()
	}
	wg.Wait()

	got := run.maxConcurrency()
	if got < 2 {
		t.Fatalf("expected non-VM calls to run in parallel (max concurrency >= 2), got %d", got)
	}
	if got > maxConcurrentVBox {
		t.Fatalf("expected non-VM calls to respect the global cap %d, got %d", maxConcurrentVBox, got)
	}
}
