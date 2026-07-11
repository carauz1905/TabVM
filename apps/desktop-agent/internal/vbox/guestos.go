package vbox

import (
	"context"
	"strings"

	"github.com/tabvm/desktop-agent/internal/models"
)

// VmGuestOS reports a VM's declared guest OS type and whether the serial-console
// terminal can be offered for it. The ostype is VirtualBox metadata, so this
// works without Guest Additions and regardless of VM power state.
func (s *service) VmGuestOS(ctx context.Context, id string) (models.VmGuestOSResponse, error) {
	if !IsValidVmID(id) {
		return models.VmGuestOSResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmGuestOSResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading guest OS")
	if err != nil {
		return models.VmGuestOSResponse{}, err
	}

	osType := parseGuestOSType(info)
	family := guestFamily(osType)
	return models.VmGuestOSResponse{
		ID:              id,
		OSType:          osType,
		Family:          family,
		TerminalCapable: guestTerminalCapable(family),
	}, nil
}

// parseGuestOSType extracts the ostype value from the machine-readable output of
// `VBoxManage showvminfo <id> --machinereadable`. It is the VirtualBox guest OS
// type identifier (e.g. "Ubuntu_64", "Windows11_64"), which is declared metadata
// and available without Guest Additions.
func parseGuestOSType(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "ostype="
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		return strings.Trim(strings.TrimPrefix(line, prefix), `"`)
	}
	return ""
}

// linuxOSTypeMarkers are lowercase substrings of VirtualBox Linux ostype IDs.
// The serial-console terminal feature targets Linux guests, so classification
// only needs to be reliable enough to gate that path.
var linuxOSTypeMarkers = []string{
	"linux", "ubuntu", "debian", "fedora", "redhat", "gentoo", "arch",
	"opensuse", "suse", "mandriva", "turbolinux", "xandros", "oracle",
	"rhel", "centos", "rocky", "alma", "kali", "mint", "zorin",
}

// guestFamily classifies a VirtualBox ostype ID into a coarse family:
// "linux", "windows", "other", or "" when the type is unknown/empty.
func guestFamily(osType string) string {
	s := strings.ToLower(strings.TrimSpace(osType))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "windows") {
		return "windows"
	}
	for _, marker := range linuxOSTypeMarkers {
		if strings.Contains(s, marker) {
			return "linux"
		}
	}
	return "other"
}

// guestTerminalCapable reports whether the serial-console terminal can be
// offered for a guest of the given family. Only Linux guests expose a real
// login TTY over the serial port (Windows serial is SAC, not a usable shell).
func guestTerminalCapable(family string) bool {
	return family == "linux"
}
