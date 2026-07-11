//go:build !windows

package hostpick

import (
	"context"
	"errors"
)

// PickFolder is unavailable off Windows; TabVM targets a Windows host agent.
func PickFolder(_ context.Context) (string, error) {
	return "", errors.New("native folder picker is only available on Windows")
}

// PickFile is unavailable off Windows; TabVM targets a Windows host agent.
func PickFile(_ context.Context) (string, error) {
	return "", errors.New("native file picker is only available on Windows")
}
