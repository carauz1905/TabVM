//go:build !windows

package main

import "log/slog"

// runTray is a no-op on non-Windows platforms. Returning false tells the caller
// to keep serving without a tray.
func runTray(*slog.Logger) bool { return false }
