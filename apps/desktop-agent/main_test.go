package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLogWriter_CreatesAndAppends(t *testing.T) {
	dir := t.TempDir()

	writer, closeLog, err := openLogWriter(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer closeLog()

	logPath := filepath.Join(dir, "logs", "agent.log")
	if _, statErr := os.Stat(logPath); statErr != nil {
		t.Fatalf("expected log file at %s: %v", logPath, statErr)
	}

	if _, writeErr := writer.Write([]byte("agent log line\n")); writeErr != nil {
		t.Fatalf("unexpected write error: %v", writeErr)
	}

	contents, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("unexpected read error: %v", readErr)
	}
	if !strings.Contains(string(contents), "agent log line") {
		t.Fatalf("expected written bytes in log file, got %q", string(contents))
	}
}

func TestOpenLogWriter_AppendsAcrossOpens(t *testing.T) {
	dir := t.TempDir()

	writer1, close1, err := openLogWriter(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := writer1.Write([]byte("first line\n")); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if err := close1(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	writer2, close2, err := openLogWriter(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer close2()
	if _, err := writer2.Write([]byte("second line\n")); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(dir, "logs", "agent.log"))
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	got := string(contents)
	if !strings.Contains(got, "first line") || !strings.Contains(got, "second line") {
		t.Fatalf("expected appended content across opens, got %q", got)
	}
}
