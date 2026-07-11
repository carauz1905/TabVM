package vbox

import (
	"strings"

	"github.com/tabvm/desktop-agent/internal/models"
)

// parseListVmsOutput parses the output of `VBoxManage list vms`.
// Expected line format: "Name" {uuid}
func parseListVmsOutput(output string) []models.VmInfo {
	vms := make([]models.VmInfo, 0)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue
		}

		if line[0] != '"' {
			continue
		}

		nameEnd := strings.IndexByte(line[1:], '"')
		if nameEnd < 0 {
			continue
		}
		nameEnd++ // adjust for the offset line[1:]

		name := line[1:nameEnd]
		remainder := line[nameEnd+1:]

		idStart := strings.IndexByte(remainder, '{')
		idEnd := strings.IndexByte(remainder, '}')
		var id string
		if idStart >= 0 && idEnd > idStart {
			id = remainder[idStart+1 : idEnd]
		}

		vms = append(vms, models.VmInfo{
			ID:    id,
			Name:  name,
			State: "listed",
		})
	}

	return vms
}

// parseRunningVmIDs parses the output of `VBoxManage list runningvms` and
// returns the VM identifiers found in each line.
func parseRunningVmIDs(output string) []string {
	ids := make([]string, 0)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue
		}

		idStart := strings.IndexByte(line, '{')
		idEnd := strings.IndexByte(line, '}')
		if idStart < 0 || idEnd <= idStart {
			continue
		}

		id := line[idStart+1 : idEnd]
		if id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// parseVmState extracts the VMState value from the machine-readable output of
// `VBoxManage showvminfo <id> --machinereadable`.
func parseVmState(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "VMState="
		if !strings.HasPrefix(line, prefix) {
			continue
		}

		value := strings.TrimPrefix(line, prefix)
		value = strings.Trim(value, `"`)
		return value
	}

	return ""
}

// normalizeVmState maps a raw VirtualBox VMState token to the stable vocabulary
// the web UI renders. VirtualBox has no distinct "booting" state, so the
// transient "starting" state is surfaced as "booting" for students. Unknown
// tokens pass through lowercased so new VirtualBox states still display. The
// canonical value "running" is preserved exactly because the UI keys the live
// console button off it.
func normalizeVmState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "running":
		return "running"
	case "starting":
		return "booting"
	case "restoring":
		return "resuming"
	case "paused":
		return "paused"
	case "saving":
		return "saving"
	case "saved":
		return "saved"
	case "stopping":
		return "stopping"
	case "poweroff":
		return "powered off"
	case "aborted":
		return "aborted"
	case "stuck":
		return "stuck"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

// vrdeInfo holds the remote display properties parsed from showvminfo output.
type vrdeInfo struct {
	enabled bool
	address string
	port    string
}

// parseVRDEInfo extracts the VRDE enabled state, address, and port from the
// machine-readable output of `VBoxManage showvminfo <id> --machinereadable`.
func parseVRDEInfo(output string) vrdeInfo {
	var info vrdeInfo

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		switch key {
		case "vrde":
			info.enabled = strings.EqualFold(value, "on") || strings.EqualFold(value, "true")
		case "vrdeaddress":
			info.address = value
		case "vrdeport":
			info.port = value
		}
	}

	return info
}
