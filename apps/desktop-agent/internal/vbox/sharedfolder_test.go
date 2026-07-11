package vbox

import (
	"context"
	"io/fs"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/runner"
)

// fakeDirInfo is an os.FileInfo reporting a directory, used to stub statPath.
type fakeDirInfo struct{ dir bool }

func (f fakeDirInfo) Name() string       { return "share" }
func (f fakeDirInfo) Size() int64        { return 0 }
func (f fakeDirInfo) Mode() fs.FileMode  { return 0 }
func (f fakeDirInfo) ModTime() time.Time { return time.Time{} }
func (f fakeDirInfo) IsDir() bool        { return f.dir }
func (f fakeDirInfo) Sys() any           { return nil }

// stubStatPathDir makes statPath report the given host path as an existing
// directory for the duration of the test.
func stubStatPathDir(t *testing.T, wantPath string) {
	t.Helper()
	original := statPath
	statPath = func(name string) (os.FileInfo, error) {
		if name == wantPath {
			return fakeDirInfo{dir: true}, nil
		}
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { statPath = original })
}

func TestListSharedFolders_ParsesMappings(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"
SharedFolderNameMachineMapping1="labshare"
SharedFolderPathMachineMapping1="C:\labs\share"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.ListSharedFolders(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Folders) != 1 {
		t.Fatalf("expected 1 shared folder, got %d", len(resp.Folders))
	}
	if resp.Folders[0].Name != "labshare" || resp.Folders[0].HostPath != `C:\labs\share` || resp.Folders[0].Transient {
		t.Fatalf("unexpected folder: %+v", resp.Folders[0])
	}
}

func TestAddSharedFolder_PersistentWhenStopped(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	hostPath := `C:\labs\share`
	stubStatPathDir(t, hostPath)
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable":                            {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " sharedfolder add " + id + " --name labshare --hostpath " + hostPath + " --automount": {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AddSharedFolder(context.Background(), id, "labshare", hostPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestAddSharedFolder_TransientWhenRunning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	hostPath := `C:\labs\share`
	stubStatPathDir(t, hostPath)
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable":                                             {ExitCode: 0, StandardOutput: `VMState="running"`},
			path + " sharedfolder add " + id + " --name labshare --hostpath " + hostPath + " --transient --automount": {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AddSharedFolder(context.Background(), id, "labshare", hostPath)
	if err != nil {
		t.Fatalf("unexpected error (transient add command may not have matched): %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestAddSharedFolder_RejectsInvalidName(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AddSharedFolder(context.Background(), id, "../evil", `C:\labs\share`)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for bad name, got %T: %v", err, err)
	}
}

func TestAddSharedFolder_RejectsTraversalPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AddSharedFolder(context.Background(), id, "labshare", `C:\labs\..\Windows`)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for traversal path, got %T: %v", err, err)
	}
}

func TestRemoveSharedFolder_UsesTransientFlagFromConfig(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"
SharedFolderNameTransientMapping1="scratch"
SharedFolderPathTransientMapping1="C:\temp\scratch"`},
			path + " sharedfolder remove " + id + " --name scratch --transient": {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.RemoveSharedFolder(context.Background(), id, "scratch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestRemoveSharedFolder_NotFound(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.RemoveSharedFolder(context.Background(), id, "missing")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for missing folder, got %T: %v", err, err)
	}
}
