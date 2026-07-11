package vbox

import (
	"context"
	"strings"

	"github.com/tabvm/desktop-agent/internal/models"
)

// clipboardModes are the shared-clipboard directions VirtualBox accepts. Sharing
// the clipboard exposes copied content across the host/guest boundary, so the
// mode is validated strictly before it reaches VBoxManage.
var clipboardModes = map[string]struct{}{
	"disabled":      {},
	"hosttoguest":   {},
	"guesttohost":   {},
	"bidirectional": {},
}

// parseClipboardMode extracts the configured shared-clipboard mode from
// machine-readable showvminfo output. VirtualBox emits it as clipboard="<mode>".
// A missing key is reported as "disabled" (the VirtualBox default).
func parseClipboardMode(output string) string {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		if key == "clipboard" || key == "clipboardmode" {
			mode := strings.ToLower(strings.TrimSpace(value))
			if mode == "" {
				return "disabled"
			}
			return mode
		}
	}
	return "disabled"
}

// GetClipboardMode returns the VM's configured shared-clipboard mode.
func (s *service) GetClipboardMode(ctx context.Context, id string) (models.ClipboardModeResponse, error) {
	if !IsValidVmID(id) {
		return models.ClipboardModeResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.ClipboardModeResponse{}, err
	}
	info, err := s.readShowVmInfo(ctx, path, id, "reading clipboard mode")
	if err != nil {
		return models.ClipboardModeResponse{}, err
	}
	return models.ClipboardModeResponse{ID: id, Mode: parseClipboardMode(info)}, nil
}

// SetClipboardMode changes the VM's shared-clipboard mode. A running VM is
// updated live with `controlvm ... clipboard mode`; a stopped VM has its saved
// configuration changed with `modifyvm --clipboard-mode`. Bidirectional (and any
// non-disabled) sharing also requires active Guest Additions in the guest to
// actually move clipboard content.
func (s *service) SetClipboardMode(ctx context.Context, id, mode string) (models.ClipboardModeResponse, error) {
	if !IsValidVmID(id) {
		return models.ClipboardModeResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if _, ok := clipboardModes[normalized]; !ok {
		return models.ClipboardModeResponse{}, &ValidationError{
			Message: "Clipboard mode must be one of: disabled, hosttoguest, guesttohost, bidirectional.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "clipboard.set", false, "VirtualBox/VBoxManage not discovered.")
		return models.ClipboardModeResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for clipboard change")
	if err != nil {
		return models.ClipboardModeResponse{}, err
	}

	var args []string
	if vmStateIsLive(parseVmState(info)) {
		args = setClipboardControlArgs(id, normalized)
	} else {
		args = setClipboardModifyArgs(id, normalized)
	}

	if err := s.runControlCommand(ctx, path, args, "setting clipboard mode"); err != nil {
		s.logOperation(ctx, id, "clipboard.set", false, "VirtualBox clipboard change failed.")
		return models.ClipboardModeResponse{}, err
	}

	s.logOperation(ctx, id, "clipboard.set", true, "")
	return models.ClipboardModeResponse{ID: id, Mode: normalized}, nil
}

// setClipboardModifyArgs changes the persisted clipboard mode on a stopped VM.
func setClipboardModifyArgs(id, mode string) []string {
	return []string{"modifyvm", id, "--clipboard-mode", mode}
}

// setClipboardControlArgs changes the clipboard mode on a running VM at runtime.
func setClipboardControlArgs(id, mode string) []string {
	return []string{"controlvm", id, "clipboard", "mode", mode}
}
