package vbox

import (
	"context"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// maxGuestOutputBytes caps the guest command output returned to the UI so a
// runaway command (e.g. `cat` of a huge file) cannot flood the response or the
// browser. Output beyond the cap is dropped and a marker is appended.
const maxGuestOutputBytes = 64 * 1024

// guestRunTimeoutMS is the guest-side process timeout passed to VBoxManage (60s);
// guestRunTimeout is the slightly longer host-side bound on the whole VBoxManage
// invocation so VBoxManage's own timeout handling wins over the runner's.
const (
	guestRunTimeoutMS = "60000"
	guestRunTimeout   = 65 * time.Second
	// copyFromTimeout bounds a guest→host file copy. Copying a large file can take
	// a while, so it gets a few minutes (mirroring copyto).
	copyFromTimeout = 5 * time.Minute
)

// RunInGuest runs a program inside a running Linux guest via VBoxManage guest
// control and returns its exit code and (capped) output. It requires the VM to
// be running with Guest Additions active and a guest username/password, which
// are used once and never stored. The password travels via a 0600 --passwordfile
// temp file, never on the command line. A non-zero *guest* exit code is a
// completed run (reported with Success=true), not a transport failure.
func (s *service) RunInGuest(ctx context.Context, id, exe string, args []string, username, password string) (models.VmGuestRunResponse, error) {
	if !IsValidVmID(id) {
		return models.VmGuestRunResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	// Guest control always needs credentials, so an empty credential set is a
	// needs-credentials response (mirroring copyto) rather than a hard error.
	if strings.TrimSpace(username) == "" || password == "" {
		return models.VmGuestRunResponse{
			Success:             false,
			VMID:                id,
			CredentialsRequired: true,
			Message:             "Running a command in the guest needs the guest username and password.",
		}, nil
	}
	if !isPlausibleGuestUsername(username) {
		return models.VmGuestRunResponse{}, &ValidationError{Message: "Guest username contains unsupported characters."}
	}
	if err := validateGuestExe(exe); err != nil {
		return models.VmGuestRunResponse{}, err
	}
	if err := validateGuestArgs(args); err != nil {
		return models.VmGuestRunResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.guest.run", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmGuestRunResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state before running a guest command")
	if err != nil {
		return models.VmGuestRunResponse{}, err
	}
	if !vmStateIsLive(parseVmState(info)) {
		return models.VmGuestRunResponse{}, &ValidationError{Message: "The VM must be running with Guest Additions active to run a command in it."}
	}

	pwPath, err := writeCredentialFile(password)
	if err != nil {
		return models.VmGuestRunResponse{}, err
	}
	defer os.Remove(pwPath)

	// runErr is intentionally ignored for the pass/fail decision: the real runner
	// returns an error for any non-zero exit, but a non-zero *guest* exit code is
	// a legitimate, completed run we want to surface. A negative exit code means
	// VBoxManage could not run the process at all (spawn failure / timeout /
	// cancellation), which is the only hard failure.
	result, _ := s.runForVM(ctx, id, path, guestControlRunArgs(id, username, pwPath, strings.TrimSpace(exe), args), guestRunTimeout)
	if result.ExitCode < 0 {
		s.logOperation(ctx, id, "vm.guest.run", false, "Guest command could not be executed.")
		return models.VmGuestRunResponse{
			Success:  false,
			VMID:     id,
			ExitCode: result.ExitCode,
			Message:  "Could not run the command in the guest. Check the username/password and that this is a running Linux guest with Guest Additions active.",
		}, nil
	}

	output, truncated := capGuestOutput(combinedOutput(result.StandardOutput, result.StandardError))
	s.logOperation(ctx, id, "vm.guest.run", true, "")
	return models.VmGuestRunResponse{
		Success:   true,
		VMID:      id,
		ExitCode:  result.ExitCode,
		Output:    output,
		Truncated: truncated,
		Message:   fmt.Sprintf("Command finished with exit code %d.", result.ExitCode),
	}, nil
}

// CopyFromGuest copies a file out of a running Linux guest into a host directory
// via VBoxManage guest control. It requires the VM to be running with Guest
// Additions active and guest credentials (used once, never stored). The host
// destination is <hostDir>/<basename(guestPath)>; it refuses to overwrite an
// existing file. The password travels via a 0600 --passwordfile temp file.
func (s *service) CopyFromGuest(ctx context.Context, id, guestPath, hostDir, username, password string) (models.VmGuestCopyFromResponse, error) {
	if !IsValidVmID(id) {
		return models.VmGuestCopyFromResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	// There is no shared-folder fast path when copying OUT of the guest, so guest
	// control (and therefore credentials) is always required.
	if strings.TrimSpace(username) == "" || password == "" {
		return models.VmGuestCopyFromResponse{
			Success:             false,
			VMID:                id,
			CredentialsRequired: true,
			Message:             "Copying a file out of the guest needs the guest username and password.",
		}, nil
	}
	if !isPlausibleGuestUsername(username) {
		return models.VmGuestCopyFromResponse{}, &ValidationError{Message: "Guest username contains unsupported characters."}
	}
	if err := validateGuestPath(guestPath); err != nil {
		return models.VmGuestCopyFromResponse{}, err
	}
	hostDir = strings.TrimSpace(hostDir)
	if err := validateExportDir(hostDir); err != nil {
		return models.VmGuestCopyFromResponse{}, err
	}

	base := guestPathBase(guestPath)
	if base == "" {
		return models.VmGuestCopyFromResponse{}, &ValidationError{Message: "Could not determine the file name from the guest path."}
	}
	hostDst := filepath.Join(hostDir, base)
	if _, statErr := statPath(hostDst); statErr == nil {
		return models.VmGuestCopyFromResponse{}, &ValidationError{
			Message: fmt.Sprintf("A file named %q already exists in the destination folder. Choose another folder or remove it first.", base),
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.guest.copyfrom", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmGuestCopyFromResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state before copying from the guest")
	if err != nil {
		return models.VmGuestCopyFromResponse{}, err
	}
	if !vmStateIsLive(parseVmState(info)) {
		return models.VmGuestCopyFromResponse{}, &ValidationError{Message: "The VM must be running with Guest Additions active to copy a file out of it."}
	}

	pwPath, err := writeCredentialFile(password)
	if err != nil {
		return models.VmGuestCopyFromResponse{}, err
	}
	defer os.Remove(pwPath)

	result, runErr := s.runForVM(ctx, id, path, guestControlCopyFromArgs(id, username, pwPath, strings.TrimSpace(guestPath), hostDst), copyFromTimeout)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "vm.guest.copyfrom", false, "Guest control copy from failed.")
		return models.VmGuestCopyFromResponse{
			Success: false,
			VMID:    id,
			Message: "Could not copy the file out of the guest. Check the username/password, that the guest path exists, and that this is a running Linux guest with Guest Additions active.",
		}, nil
	}

	s.logOperation(ctx, id, "vm.guest.copyfrom", true, "")
	return models.VmGuestCopyFromResponse{
		Success:  true,
		VMID:     id,
		HostPath: hostDst,
		Message:  fmt.Sprintf("Copied %q from the guest to %s.", base, hostDst),
	}, nil
}

// guestControlRunArgs builds the VBoxManage command that runs a program in the
// guest. The password travels via --passwordfile, never in argv. Only
// --wait-stdout is used: combining it with --wait-stderr triggers VERR_DUPLICATE
// on this VBoxManage version (see the getty / GA arg builders). On VirtualBox
// 7.x `run` sets argv[0] to the --exe value itself, so the tokens after "--" are
// argv[1..] — the program name is NOT repeated (matching guestControlMkdirArgs).
func guestControlRunArgs(id, username, pwPath, exe string, args []string) []string {
	out := []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwPath,
		"run",
		"--exe", exe,
		"--timeout", guestRunTimeoutMS,
		"--wait-stdout",
		"--",
	}
	return append(out, args...)
}

// guestControlCopyFromArgs builds the VBoxManage command that copies a guest
// file to the host. It mirrors copyto (source then destination), with the guest
// path as the source and the host path as the destination. Credentials travel
// via --passwordfile.
func guestControlCopyFromArgs(id, username, pwPath, guestSrc, hostDst string) []string {
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwPath,
		"copyfrom", guestSrc, hostDst,
	}
}

// validateGuestExe checks the program path before it is handed to VBoxManage's
// --exe option: it must be a non-empty absolute POSIX path with no control
// characters and no comma (VBoxManage guest control treats commas specially in
// some argument contexts). exec.Command bypasses the shell, so this is
// defence-in-depth, not the sole protection.
func validateGuestExe(exe string) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return &ValidationError{Message: "A guest command (absolute path) is required."}
	}
	if !strings.HasPrefix(exe, "/") {
		return &ValidationError{Message: "The guest command must be an absolute path (for example /bin/ls)."}
	}
	if containsControlChar(exe) {
		return &ValidationError{Message: "The guest command must not contain control characters."}
	}
	if strings.Contains(exe, ",") {
		return &ValidationError{Message: "The guest command must not contain commas."}
	}
	return nil
}

// validateGuestArgs rejects control characters in any argument; everything else
// is passed through verbatim (exec.Command bypasses the shell).
func validateGuestArgs(args []string) error {
	for _, a := range args {
		if containsControlChar(a) {
			return &ValidationError{Message: "Command arguments must not contain control characters."}
		}
	}
	return nil
}

// validateGuestPath checks the guest file path for a copy-out: it must be a
// non-empty absolute POSIX path with no control characters and no ".." segments.
func validateGuestPath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return &ValidationError{Message: "A guest file path is required."}
	}
	if !strings.HasPrefix(p, "/") {
		return &ValidationError{Message: "The guest file path must be absolute (start with '/')."}
	}
	if containsControlChar(p) {
		return &ValidationError{Message: "The guest file path must not contain control characters."}
	}
	if containsTraversal(p) {
		return &ValidationError{Message: "The guest file path must not contain '..' segments."}
	}
	return nil
}

// guestPathBase returns the POSIX basename of a guest path, or "" when no usable
// file name remains (e.g. the path is "/" or a directory). It uses path.Base
// (not filepath.Base) because guest paths are always POSIX regardless of the
// host OS.
func guestPathBase(p string) string {
	p = strings.TrimRight(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	base := pathpkg.Base(p)
	if base == "." || base == "/" || base == ".." {
		return ""
	}
	return base
}

// capGuestOutput truncates guest output to maxGuestOutputBytes and reports
// whether it was cut. The head of the output is kept (the most relevant part for
// most commands) and a marker is appended. The cut is trimmed back to a valid
// UTF-8 boundary so a split multi-byte rune never corrupts the JSON response.
func capGuestOutput(s string) (string, bool) {
	if len(s) <= maxGuestOutputBytes {
		return s, false
	}
	head := strings.ToValidUTF8(s[:maxGuestOutputBytes], "")
	return head + "\n... [output truncated]", true
}
