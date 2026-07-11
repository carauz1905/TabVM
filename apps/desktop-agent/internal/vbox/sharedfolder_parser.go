package vbox

import (
	"sort"
	"strconv"
	"strings"
)

// sharedFolderInfo describes one host directory shared into a guest, parsed from
// `VBoxManage showvminfo <id> --machinereadable`. VirtualBox reports three mapping
// scopes: machine (persistent, per-VM), global (persistent, shared across all VMs),
// and transient (session-only, lost on power-off). Each scope needs its matching
// flag on removal: none for machine, --global for global, --transient for transient.
type sharedFolderInfo struct {
	name      string
	hostPath  string
	transient bool
	global    bool
}

// sharedFolder mapping key prefixes emitted by --machinereadable.
const (
	sfNameMachinePrefix   = "SharedFolderNameMachineMapping"
	sfPathMachinePrefix   = "SharedFolderPathMachineMapping"
	sfNameGlobalPrefix    = "SharedFolderNameGlobalMapping"
	sfPathGlobalPrefix    = "SharedFolderPathGlobalMapping"
	sfNameTransientPrefix = "SharedFolderNameTransientMapping"
	sfPathTransientPrefix = "SharedFolderPathTransientMapping"
)

// parseSharedFolders extracts the persistent (machine) and transient shared
// folders configured on a VM from machine-readable showvminfo output. Persistent
// mappings are listed first, then transient ones, each ordered by their mapping
// index so the UI shows a stable order.
func parseSharedFolders(output string) []sharedFolderInfo {
	machineNames := map[int]string{}
	machinePaths := map[int]string{}
	globalNames := map[int]string{}
	globalPaths := map[int]string{}
	transientNames := map[int]string{}
	transientPaths := map[int]string{}

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadableRawKey(line)
		if !ok {
			continue
		}
		if idx, ok := sharedFolderIndex(key, sfNameMachinePrefix); ok {
			machineNames[idx] = value
		} else if idx, ok := sharedFolderIndex(key, sfPathMachinePrefix); ok {
			machinePaths[idx] = value
		} else if idx, ok := sharedFolderIndex(key, sfNameGlobalPrefix); ok {
			globalNames[idx] = value
		} else if idx, ok := sharedFolderIndex(key, sfPathGlobalPrefix); ok {
			globalPaths[idx] = value
		} else if idx, ok := sharedFolderIndex(key, sfNameTransientPrefix); ok {
			transientNames[idx] = value
		} else if idx, ok := sharedFolderIndex(key, sfPathTransientPrefix); ok {
			transientPaths[idx] = value
		}
	}

	folders := make([]sharedFolderInfo, 0, len(machineNames)+len(globalNames)+len(transientNames))
	folders = append(folders, collectSharedFolders(machineNames, machinePaths, false, false)...)
	folders = append(folders, collectSharedFolders(globalNames, globalPaths, false, true)...)
	folders = append(folders, collectSharedFolders(transientNames, transientPaths, true, false)...)
	return folders
}

// collectSharedFolders pairs names with paths by mapping index and returns them
// ordered by index. A mapping is skipped when it has no name.
func collectSharedFolders(names, paths map[int]string, transient, global bool) []sharedFolderInfo {
	indices := make([]int, 0, len(names))
	for idx := range names {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	folders := make([]sharedFolderInfo, 0, len(indices))
	for _, idx := range indices {
		name := strings.TrimSpace(names[idx])
		if name == "" {
			continue
		}
		folders = append(folders, sharedFolderInfo{
			name:      name,
			hostPath:  strings.TrimSpace(paths[idx]),
			transient: transient,
			global:    global,
		})
	}
	return folders
}

// sharedFolderIndex reports whether key is prefix followed by a positive integer
// mapping index (e.g. "SharedFolderNameMachineMapping1") and returns that index.
func sharedFolderIndex(key, prefix string) (int, bool) {
	if !strings.HasPrefix(key, prefix) {
		return 0, false
	}
	suffix := key[len(prefix):]
	if suffix == "" {
		return 0, false
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
