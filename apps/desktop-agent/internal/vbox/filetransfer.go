package vbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// TransferFileToGuest copies an uploaded file into a running guest. It uses the
// least-friction mechanism available (hybrid):
//
//  1. If the VM has a shared folder, the bytes are written straight into that
//     folder's host directory, so the file appears in the guest at
//     /media/sf_<name> immediately — no guest credentials required.
//  2. Otherwise it falls back to VBoxManage guest control (copyto), which needs
//     the guest username/password. When those are absent the response sets
//     CredentialsRequired so the UI can prompt and retry.
//
// The filename is sanitized to its basename before it touches the host
// filesystem or a guest path, so an uploaded name can never escape the target
// directory via path traversal.
func (s *service) TransferFileToGuest(ctx context.Context, id, filename string, data []byte, username, password string) (models.VmFileTransferResponse, error) {
	if !IsValidVmID(id) {
		return models.VmFileTransferResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if len(data) == 0 {
		return models.VmFileTransferResponse{}, &ValidationError{Message: "No file content was uploaded."}
	}
	safeName, err := sanitizeTransferFilename(filename)
	if err != nil {
		return models.VmFileTransferResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmFileTransferResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading shared folders for file transfer")
	if err != nil {
		return models.VmFileTransferResponse{}, err
	}

	// Preferred path: write into an existing shared folder's host directory.
	if share, ok := chooseWritableShare(parseSharedFolders(info)); ok {
		dest := filepath.Join(share.hostPath, safeName)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			s.logOperation(ctx, id, "file.transfer", false, "Writing into shared folder failed.")
			return models.VmFileTransferResponse{}, &ExecutionError{Message: fmt.Sprintf("writing to shared folder: %v", err)}
		}
		s.logOperation(ctx, id, "file.transfer", true, "")
		return models.VmFileTransferResponse{
			Success:   true,
			VMID:      id,
			Method:    "shared-folder",
			GuestPath: "/media/sf_" + share.name + "/" + safeName,
			Message:   fmt.Sprintf("%q is in shared folder %q (guest: /media/sf_%s).", safeName, share.name, share.name),
		}, nil
	}

	// Fallback: guest control copy needs credentials.
	if strings.TrimSpace(username) == "" || password == "" {
		return models.VmFileTransferResponse{
			Success:             false,
			VMID:                id,
			CredentialsRequired: true,
			Message:             "This VM has no shared folder, so copying a file in needs the guest username and password.",
		}, nil
	}
	if !isPlausibleGuestUsername(username) {
		return models.VmFileTransferResponse{}, &ValidationError{Message: "Guest username contains unsupported characters."}
	}
	if !vmStateIsLive(parseVmState(info)) {
		return models.VmFileTransferResponse{}, &ValidationError{Message: "The VM must be running to copy files into it."}
	}

	// Stage the upload as a host temp file (copyto takes a source path), and the
	// password as a 0600 temp file so it never appears on the command line.
	tmpPath, err := writeTempFile("tabvm-xfer-*", data)
	if err != nil {
		return models.VmFileTransferResponse{}, fmt.Errorf("staging upload: %w", err)
	}
	defer os.Remove(tmpPath)

	pwPath, err := writeCredentialFile(password)
	if err != nil {
		return models.VmFileTransferResponse{}, err
	}
	defer os.Remove(pwPath)

	guestDir := guestHomeDir(username) + "/TabVM"
	guestDst := guestDir + "/" + safeName

	// Ensure the destination directory exists (best-effort).
	_, _ = s.runForVM(ctx, id, path, guestControlMkdirArgs(id, username, pwPath, guestDir), 30*time.Second)

	result, runErr := s.runForVM(ctx, id, path, guestControlCopyToArgs(id, username, pwPath, tmpPath, guestDst), 5*time.Minute)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "file.transfer", false, "Guest control copy failed.")
		return models.VmFileTransferResponse{
			Success: false,
			VMID:    id,
			Method:  "guest-control",
			Message: "Could not copy the file into the guest. Check the username/password and that the guest is a running Linux VM with Guest Additions active.",
		}, nil
	}

	s.logOperation(ctx, id, "file.transfer", true, "")
	return models.VmFileTransferResponse{
		Success:   true,
		VMID:      id,
		Method:    "guest-control",
		GuestPath: guestDst,
		Message:   fmt.Sprintf("%q copied into the guest at %s.", safeName, guestDst),
	}, nil
}

// chooseWritableShare returns the first shared folder whose host directory
// exists, so the transfer can write straight into it.
func chooseWritableShare(folders []sharedFolderInfo) (sharedFolderInfo, bool) {
	for _, f := range folders {
		if strings.TrimSpace(f.hostPath) == "" {
			continue
		}
		if fi, err := statPath(f.hostPath); err == nil && fi.IsDir() {
			return f, true
		}
	}
	return sharedFolderInfo{}, false
}

// sanitizeTransferFilename reduces an uploaded name to a safe basename: it drops
// any directory components, rejects "." / ".." / empty, replaces characters that
// are invalid on the host (Windows) filesystem with '_', strips control
// characters, and caps the length. This makes path traversal impossible.
func sanitizeTransferFilename(name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", &ValidationError{Message: "Invalid file name."}
	}

	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			continue
		}
		switch r {
		case '<', '>', ':', '"', '|', '?', '*', '/', '\\':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	// Windows disallows trailing spaces and dots on file names.
	out := strings.TrimRight(strings.TrimSpace(b.String()), " .")
	if out == "" || out == "." || out == ".." {
		return "", &ValidationError{Message: "Invalid file name."}
	}
	if len(out) > 255 {
		out = out[:255]
	}
	return out, nil
}

// writeTempFile stages bytes into a private temp file and returns its path.
func writeTempFile(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

// writeCredentialFile writes a password to a 0600 temp file so it can be passed
// to VBoxManage via --passwordfile instead of on the command line.
func writeCredentialFile(password string) (string, error) {
	f, err := os.CreateTemp("", "tabvm-xfer-pw-*.txt")
	if err != nil {
		return "", fmt.Errorf("creating credential file: %w", err)
	}
	_ = f.Chmod(0o600)
	if _, err := f.WriteString(password); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("writing credential file: %w", err)
	}
	f.Close()
	return f.Name(), nil
}

// guestHomeDir returns the conventional Linux home directory for a guest user.
func guestHomeDir(username string) string {
	if strings.EqualFold(username, "root") {
		return "/root"
	}
	return "/home/" + username
}

func guestControlMkdirArgs(id, username, pwPath, dir string) []string {
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwPath,
		"run", "--exe", "/bin/mkdir",
		"--timeout", "30000", "--wait-stdout",
		// VBoxManage sets argv[0] to --exe (/bin/mkdir); tokens after -- are argv[1..].
		"--", "-p", dir,
	}
}

func guestControlCopyToArgs(id, username, pwPath, hostSrc, guestDst string) []string {
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwPath,
		"copyto", hostSrc, guestDst,
	}
}
