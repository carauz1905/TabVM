package vbox

import (
	"strings"

	"github.com/tabvm/desktop-agent/internal/models"
)

// parseSnapshots extracts a VM's snapshot tree from `snapshot list
// --machinereadable` output and returns it flattened in tree order with the
// current snapshot's UUID.
//
// The machine-readable format encodes the tree in the key suffix: a root
// snapshot is SnapshotName/SnapshotUUID, its first child SnapshotName-1, a
// grandchild SnapshotName-1-1, and so on. The number of "-N" segments is the
// depth. Descriptions share the same suffix (SnapshotDescription[-N...]).
func parseSnapshots(output string) ([]models.Snapshot, string) {
	type entry struct {
		name, uuid, description string
	}
	entries := map[string]*entry{}
	order := make([]string, 0, 8)
	current := ""

	get := func(suffix string) *entry {
		e := entries[suffix]
		if e == nil {
			e = &entry{}
			entries[suffix] = e
			order = append(order, suffix)
		}
		return e
	}

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadableRawKey(line)
		if !ok {
			continue
		}
		switch {
		case key == "CurrentSnapshotUUID":
			current = value
		case strings.HasPrefix(key, "SnapshotName"):
			get(strings.TrimPrefix(key, "SnapshotName")).name = value
		case strings.HasPrefix(key, "SnapshotUUID"):
			get(strings.TrimPrefix(key, "SnapshotUUID")).uuid = value
		case strings.HasPrefix(key, "SnapshotDescription"):
			// Description may arrive before name/uuid for the same suffix; get()
			// creates the entry so it is not lost.
			get(strings.TrimPrefix(key, "SnapshotDescription")).description = value
		}
	}

	snapshots := make([]models.Snapshot, 0, len(order))
	for _, suffix := range order {
		e := entries[suffix]
		if e.name == "" && e.uuid == "" {
			continue
		}
		snapshots = append(snapshots, models.Snapshot{
			Name:        e.name,
			UUID:        e.uuid,
			Description: e.description,
			Depth:       strings.Count(suffix, "-"),
			Current:     current != "" && e.uuid == current,
		})
	}
	return snapshots, current
}
