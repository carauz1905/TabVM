//go:build windows

package main

import (
	"errors"
	"strings"
	"testing"
)

func TestLaunchAction(t *testing.T) {
	tests := []struct {
		name         string
		probeErr     error
		statusOK     bool
		agentVersion string
		want         action
	}{
		{
			name:     "no agent responding starts a fresh agent",
			probeErr: errors.New("connection refused"),
			want:     actionStart,
		},
		{
			name:     "non-200 answer is treated as no agent",
			statusOK: false,
			want:     actionStart,
		},
		{
			name:         "matching version only opens the browser",
			statusOK:     true,
			agentVersion: "1.2.3",
			want:         actionOpen,
		},
		{
			name:         "different version restarts the stale agent",
			statusOK:     true,
			agentVersion: "1.0.0",
			want:         actionRestart,
		},
		{
			name:         "missing version from a very old agent restarts it",
			statusOK:     true,
			agentVersion: "",
			want:         actionRestart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := launchAction(tt.probeErr, tt.statusOK, tt.agentVersion, "1.2.3")
			if got != tt.want {
				t.Errorf("launchAction(%v, %v, %q, %q) = %v, want %v",
					tt.probeErr, tt.statusOK, tt.agentVersion, "1.2.3", got, tt.want)
			}
		})
	}
}

func TestFailureMessage(t *testing.T) {
	got := failureMessage("the agent did not start", `C:\logs\agent.log`)

	for _, want := range []string{
		"TabVM could not start",
		"the agent did not start",
		`C:\logs\agent.log`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("failureMessage(...) = %q, missing %q", got, want)
		}
	}
}
