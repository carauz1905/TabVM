package store

import "time"

// VmConsolePort records the console port assigned to a VM so it can remain
// stable across agent restarts.
type VmConsolePort struct {
	VMID      string
	Port      int
	Address   string
	Protocol  string
	Source    string
	UpdatedAt time.Time
}

// OperationLogEntry records a lifecycle or console preparation action for
// audit purposes. Entries are bounded by retention cleanup.
type OperationLogEntry struct {
	ID         int64
	VMID       string
	Action     string
	Success    bool
	Message    string
	RecordedAt time.Time
}
