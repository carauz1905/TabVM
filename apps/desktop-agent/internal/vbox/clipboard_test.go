package vbox

import (
	"context"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestParseClipboardMode(t *testing.T) {
	cases := map[string]string{
		`clipboard="bidirectional"`: "bidirectional",
		`clipboard="disabled"`:      "disabled",
		`clipboard="hosttoguest"`:   "hosttoguest",
		`name="lab"`:                "disabled", // missing key -> default
	}
	for output, want := range cases {
		if got := parseClipboardMode(output); got != want {
			t.Errorf("parseClipboardMode(%q) = %q, want %q", output, got, want)
		}
	}
}

func TestGetClipboardMode(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"
clipboard="bidirectional"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.GetClipboardMode(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Mode != "bidirectional" {
		t.Fatalf("expected bidirectional, got %q", resp.Mode)
	}
}

func TestSetClipboardMode_ModifyWhenStopped(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable":            {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " modifyvm " + id + " --clipboard-mode bidirectional": {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.SetClipboardMode(context.Background(), id, "bidirectional")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Mode != "bidirectional" {
		t.Fatalf("expected bidirectional, got %q", resp.Mode)
	}
}

func TestSetClipboardMode_ControlvmWhenRunning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable":         {ExitCode: 0, StandardOutput: `VMState="running"`},
			path + " controlvm " + id + " clipboard mode guesttohost": {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.SetClipboardMode(context.Background(), id, "guesttohost")
	if err != nil {
		t.Fatalf("unexpected error (controlvm form may not have matched): %v", err)
	}
	if resp.Mode != "guesttohost" {
		t.Fatalf("expected guesttohost, got %q", resp.Mode)
	}
}

func TestSetClipboardMode_RejectsInvalid(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.SetClipboardMode(context.Background(), id, "everything")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for invalid mode, got %T: %v", err, err)
	}
}
