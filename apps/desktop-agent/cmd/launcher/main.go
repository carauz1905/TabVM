//go:build windows

// Command launcher is the double-click entry point for TabVM. It starts the
// agent in the background (no console window) if it is not already running, then
// opens the default browser at the splash screen. If the agent is already up it
// just opens a new browser tab, so launching again is safe and cheap.
package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	healthURL = "http://127.0.0.1:5230/health"
	openURL   = "http://127.0.0.1:5230/?splash=1"
	agentExe  = "tabvm-agent.exe"
	// createNoWindow keeps spawned processes from flashing a console window.
	createNoWindow = 0x08000000
)

func main() {
	if !agentHealthy() {
		_ = startAgent()
		waitForAgent(30 * time.Second)
	}
	openBrowser(openURL)
}

// agentHealthy reports whether an agent is already answering on the loopback
// port, so a second launch opens a tab instead of starting a duplicate.
func agentHealthy() bool {
	client := http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func waitForAgent(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if agentHealthy() {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// startAgent launches the sibling agent binary detached, in Production mode, with
// its output appended to a per-user log file. The child keeps running after this
// launcher exits.
func startAgent() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	exePath := filepath.Join(filepath.Dir(self), agentExe)

	cmd := exec.Command(exePath)
	cmd.Env = append(os.Environ(), "TABVM_AGENT_ENV=Production")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if logFile, logErr := os.OpenFile(agentLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); logErr == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	return cmd.Start()
}

func agentLogPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return "tabvm-agent.log"
	}
	dir := filepath.Join(base, "TabVM")
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "agent.log")
}

// openBrowser opens the URL in the OS default browser. When a browser is already
// open this adds a new tab, which is the desired launch behavior.
func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = cmd.Start()
}
