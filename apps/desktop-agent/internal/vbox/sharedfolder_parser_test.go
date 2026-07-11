package vbox

import "testing"

func TestParseSharedFolders_MachineAndTransient(t *testing.T) {
	output := `name="lab"
VMState="running"
SharedFolderNameMachineMapping1="labshare"
SharedFolderPathMachineMapping1="C:\labs\share"
SharedFolderNameMachineMapping2="docs"
SharedFolderPathMachineMapping2="C:\Users\student\docs"
SharedFolderNameTransientMapping1="scratch"
SharedFolderPathTransientMapping1="C:\temp\scratch"
`

	folders := parseSharedFolders(output)

	if len(folders) != 3 {
		t.Fatalf("expected 3 shared folders, got %d: %+v", len(folders), folders)
	}

	// Persistent mappings come first, ordered by index.
	if folders[0].name != "labshare" || folders[0].hostPath != `C:\labs\share` || folders[0].transient {
		t.Errorf("unexpected folder[0]: %+v", folders[0])
	}
	if folders[1].name != "docs" || folders[1].transient {
		t.Errorf("unexpected folder[1]: %+v", folders[1])
	}
	if folders[2].name != "scratch" || folders[2].hostPath != `C:\temp\scratch` || !folders[2].transient {
		t.Errorf("expected transient folder[2], got: %+v", folders[2])
	}
}

func TestParseSharedFolders_Global(t *testing.T) {
	// VirtualBox can register a share under the Global scope (e.g. an --automount
	// runtime share on some hosts). It must be read and reported as persistent
	// (not transient) so it shows in the UI and survives a power cycle.
	output := `name="h4king"
VMState="running"
SharedFolderNameGlobalMapping1="Shared"
SharedFolderPathGlobalMapping1="C:\Users\admin\Desktop\Shared"
`

	folders := parseSharedFolders(output)

	if len(folders) != 1 {
		t.Fatalf("expected 1 shared folder, got %d: %+v", len(folders), folders)
	}
	f := folders[0]
	if f.name != "Shared" || f.hostPath != `C:\Users\admin\Desktop\Shared` {
		t.Errorf("unexpected folder: %+v", f)
	}
	if f.transient {
		t.Errorf("global share must not be transient: %+v", f)
	}
	if !f.global {
		t.Errorf("expected global flag set: %+v", f)
	}
}

func TestParseSharedFolders_None(t *testing.T) {
	output := `name="lab"
VMState="poweroff"
`
	folders := parseSharedFolders(output)
	if len(folders) != 0 {
		t.Fatalf("expected no shared folders, got %d", len(folders))
	}
}
