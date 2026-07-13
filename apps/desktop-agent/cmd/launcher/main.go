//go:build windows

// Command launcher is the double-click entry point for TabVM. It starts the
// agent in the background (no console window) if it is not already running, then
// opens the default browser at the splash screen. If an agent is already up but
// reports a different version (a leftover from a previous install), it is
// stopped and replaced so the browser never loads a stale embedded web UI.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/tabvm/desktop-agent/internal/version"
)

const (
	healthURL = "http://127.0.0.1:5230/health"
	openURL   = "http://127.0.0.1:5230/?splash=1"
	agentExe  = "tabvm-agent.exe"
	// createNoWindow keeps spawned processes from flashing a console window.
	createNoWindow = 0x08000000
)

// action is the launch decision derived from probing the health endpoint.
type action int

const (
	// actionStart means no agent is running: start one, then open the browser.
	actionStart action = iota
	// actionOpen means a current agent is already running: just open the browser.
	actionOpen
	// actionRestart means a stale agent from another version is running: stop
	// it, start the sibling binary, then open the browser.
	actionRestart
)

func main() {
	probe, err := probeAgent()
	switch launchAction(err, probe.ok, probe.version, version.Version) {
	case actionOpen:
		// A healthy agent of the installed version is already serving.
	case actionRestart:
		if !stopStaleAgent() {
			failLaunch("the previous TabVM agent could not be stopped; close it manually and launch again")
			return
		}
		if !startAndWait() {
			return
		}
	default: // actionStart
		if !startAndWait() {
			return
		}
	}
	openBrowser(openURL)
}

// startAndWait starts the sibling agent and waits for it to become healthy.
// On failure it records the reason and notifies the user, then reports false so
// the caller never opens a browser tab pointing at a dead endpoint.
func startAndWait() bool {
	if err := startAgent(); err != nil {
		failLaunch("the TabVM agent could not be started: " + err.Error())
		return false
	}
	if !waitForAgent(30 * time.Second) {
		failLaunch("the TabVM agent did not become healthy within 30 seconds")
		return false
	}
	return true
}

// launchAction decides what the launcher must do based on the health probe.
// Pure so it is unit-testable: probeErr is any transport error, statusOK is
// whether the agent answered HTTP 200, agentVersion is the version reported by
// /health (empty for very old agents), and ownVersion is the version this
// launcher was built with.
func launchAction(probeErr error, statusOK bool, agentVersion, ownVersion string) action {
	if probeErr != nil || !statusOK {
		return actionStart
	}
	if agentVersion == ownVersion {
		return actionOpen
	}
	// A running agent with a different (or missing) version keeps serving its
	// old embedded UI forever; it must be replaced.
	return actionRestart
}

// healthProbe is the launcher-side view of GET /health.
type healthProbe struct {
	ok      bool   // agent answered HTTP 200
	version string // version reported by the agent; empty if absent
}

// probeAgent queries the health endpoint and reports whether an agent is
// answering and which version it claims. Extra or missing JSON fields are
// tolerated so probes against much older agents still succeed.
func probeAgent() (healthProbe, error) {
	client := http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(healthURL)
	if err != nil {
		return healthProbe{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return healthProbe{}, nil
	}
	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	// A malformed body still counts as a running agent with unknown version.
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return healthProbe{ok: true, version: body.Version}, nil
}

// stopStaleAgent force-kills any running agent process, then waits until the
// health endpoint stops answering so the replacement can bind the port. It
// retries the kill once before giving up and reports whether the port was
// confirmed released, so the caller never starts the new agent into a port race.
// taskkill by image name is the only reliable path for already-deployed agents
// that expose no shutdown endpoint; the binary name is constant across installs.
func stopStaleAgent() bool {
	killAgentProcess()
	if waitForAgentGone(5 * time.Second) {
		return true
	}
	// The first kill may have raced with the process; try once more briefly.
	killAgentProcess()
	return waitForAgentGone(2 * time.Second)
}

func killAgentProcess() {
	cmd := exec.Command("taskkill", "/F", "/IM", agentExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = cmd.Run()
}

// waitForAgentGone reports whether the health endpoint stopped answering
// within the deadline, meaning the loopback port has been released.
func waitForAgentGone(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := probeAgent(); err != nil {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// waitForAgent reports whether the agent answered healthy within the deadline.
func waitForAgent(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if probe, err := probeAgent(); err == nil && probe.ok {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
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

// failLaunch records a launcher-authored failure in the shared agent log and
// shows a message box, the only user-visible surface for a windowsgui binary.
func failLaunch(reason string) {
	logPath := agentLogPath()
	logLaunchFailure(logPath, reason)
	showErrorBox(failureMessage(reason, logPath))
}

// failureMessage builds the user-facing error text. Pure so it is unit-testable.
func failureMessage(reason, logPath string) string {
	return "TabVM could not start.\n\n" + reason + "\n\nSee the log for details:\n" + logPath
}

// logLaunchFailure appends a launcher-authored line to the same log file the
// agent writes to, so launch failures are diagnosable in one place.
func logLaunchFailure(logPath, reason string) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s launcher: %s\n", time.Now().Format(time.RFC3339), reason)
}

// showErrorBox displays a native error dialog via user32 MessageBoxW.
func showErrorBox(text string) {
	const mbIconError = 0x00000010
	title, err := syscall.UTF16PtrFromString("TabVM")
	if err != nil {
		return
	}
	body, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	messageBoxW := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	_, _, _ = messageBoxW.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), mbIconError)
}
