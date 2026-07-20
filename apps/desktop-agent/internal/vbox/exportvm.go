package vbox

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// exportTimeout bounds a whole export. Exporting copies (and compresses) every
// disk image into the appliance, so it can take many minutes; callers run it on
// a background job.
const exportTimeout = 30 * time.Minute

// ValidateExport runs the synchronous preconditions for an export (valid ID, a
// valid destination directory, the source powered off, and no existing target
// file to clobber). The server calls it before starting the background export
// job so the user gets an immediate error instead of a job that fails later.
func (s *service) ValidateExport(ctx context.Context, sourceID, directory string) error {
	if !IsValidVmID(sourceID) {
		return &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateExportDir(directory); err != nil {
		return err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return err
	}
	_, err = s.checkExportSource(ctx, path, sourceID, directory)
	return err
}

// ExportVM exports a stopped VM to an .ova appliance in the destination
// directory. The output filename is derived from the (sanitized) VM name, so
// the caller only chooses a folder. It is long-running; callers run it on a
// background job. The returned response carries the written .ova path in its
// message so the UI can show where the file landed.
func (s *service) ExportVM(ctx context.Context, sourceID, directory string) (models.VmCreateResponse, error) {
	// ID and directory are validated up front, before any VBoxManage call, so a
	// bad request fails fast and identically whether or not VBoxManage is present.
	if !IsValidVmID(sourceID) {
		return models.VmCreateResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateExportDir(directory); err != nil {
		return models.VmCreateResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, sourceID, "vm.export", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmCreateResponse{}, err
	}

	outputPath, err := s.checkExportSource(ctx, path, sourceID, directory)
	if err != nil {
		return models.VmCreateResponse{}, err
	}

	if err := s.runControlCommandTimeout(ctx, path, exportVmArgs(sourceID, outputPath), "exporting VM", exportTimeout); err != nil {
		s.logOperation(ctx, sourceID, "vm.export", false, "VBoxManage export failed.")
		return models.VmCreateResponse{}, err
	}

	s.logOperation(ctx, sourceID, "vm.export", true, "")
	return models.VmCreateResponse{
		Success: true,
		VMID:    sourceID,
		Message: fmt.Sprintf("Exported to %s", outputPath),
	}, nil
}

// checkExportSource verifies the source VM can be exported (it must be powered
// off, mirroring the clone guard) and derives the output .ova path from the VM
// name. It refuses to overwrite an existing file so an export never clobbers
// another appliance.
func (s *service) checkExportSource(ctx context.Context, path, sourceID, directory string) (string, error) {
	info, err := s.readShowVmInfo(ctx, path, sourceID, "reading VM state before export")
	if err != nil {
		return "", err
	}
	if vmStateIsLive(parseVmState(info)) {
		return "", &ValidationError{Message: "The VM is running. Power it off before exporting it."}
	}

	base := sanitizeExportFileBase(parseVmName(info))
	if base == "" {
		// A name that sanitizes to nothing (empty or all-illegal) still yields a
		// stable, safe filename derived from the VM id.
		base = sourceID
	}
	outputPath := filepath.Join(directory, base+".ova")
	if _, statErr := statPath(outputPath); statErr == nil {
		return "", &ValidationError{Message: fmt.Sprintf("A file named %q already exists in the destination folder. Choose another folder or remove it first.", base+".ova")}
	}
	return outputPath, nil
}

// exportVmArgs builds the VBoxManage export command. The .ova extension selects
// the default single-file OVA appliance format.
func exportVmArgs(sourceID, outputPath string) []string {
	return []string{"export", sourceID, "--output", outputPath}
}

// parseVmName extracts the VM name from machine-readable showvminfo output
// (the `name="..."` line).
func parseVmName(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if key, value, ok := splitMachineReadableRawKey(line); ok && key == "name" {
			return value
		}
	}
	return ""
}

// sanitizeExportFileBase turns a VM name into a safe filename base, keeping only
// letters, digits, space, dot, dash and underscore and dropping everything else
// (path separators, control characters, and shell/filesystem metacharacters).
// Leading and trailing spaces and dots are trimmed. The result is empty when
// nothing usable remains, so callers fall back to the VM id.
func sanitizeExportFileBase(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), " .")
}

// validateExportDir ensures the destination is an existing, absolute,
// traversal-free directory with no control characters. Exporting writes a whole
// appliance into this directory, so the path is validated strictly before use.
func validateExportDir(directory string) error {
	trimmed := strings.TrimSpace(directory)
	if trimmed == "" {
		return &ValidationError{Message: "A destination directory is required."}
	}
	if !filepath.IsAbs(trimmed) {
		return &ValidationError{Message: "The destination directory must be an absolute path."}
	}
	if containsControlChar(trimmed) {
		return &ValidationError{Message: "The destination directory must not contain control characters."}
	}
	if containsTraversal(trimmed) {
		return &ValidationError{Message: "The destination directory must not contain '..' segments."}
	}

	info, err := statPath(trimmed)
	if err != nil {
		return &ValidationError{Message: "The destination directory does not exist or is not accessible."}
	}
	if !info.IsDir() {
		return &ValidationError{Message: "The destination must be a directory."}
	}
	return nil
}

// containsControlChar reports whether s contains any ASCII control character.
func containsControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
