package vbox

import "testing"

// Every caller-supplied value that reaches VBoxManage as an argument must be
// unable to pose as an option. exec.Command rules out the shell, but it does
// nothing about VBoxManage's own flag parser: a name of "--transient" is handed
// to it verbatim and interpreted rather than treated as text.
//
// Four of these validators already rejected a leading dash; the shared-folder
// and VM name patterns did not. This table keeps all six honest together so the
// guarantee cannot regress in one of them unnoticed.
func TestValidatorsRejectValuesThatCouldPoseAsFlags(t *testing.T) {
	flagLike := []string{"-x", "--transient", "--global", "--help", "-"}

	validators := map[string]func(string) bool{
		"validateSharedFolderName":  func(s string) bool { return validateSharedFolderName(s) == nil },
		"validateVmName":            func(s string) bool { return validateVmName(s) == nil },
		"validateSnapshotName":      func(s string) bool { return validateSnapshotName(s) == nil },
		"isPlausibleForwardingName": isPlausibleForwardingName,
		"isPlausibleHostInterface":  isPlausibleHostInterface,
		"isPlausibleGuestUsername":  isPlausibleGuestUsername,
	}

	for name, accepts := range validators {
		for _, value := range flagLike {
			t.Run(name+"/"+value, func(t *testing.T) {
				if accepts(value) {
					t.Errorf("%s accepted %q, which VBoxManage would parse as an option", name, value)
				}
			})
		}
	}
}

// The dash guard must not cost legitimate names that merely contain a dash.
func TestValidatorsStillAcceptOrdinaryNames(t *testing.T) {
	cases := []struct {
		validator string
		accepts   func(string) bool
		value     string
	}{
		{"validateSharedFolderName", func(s string) bool { return validateSharedFolderName(s) == nil }, "my-share"},
		{"validateSharedFolderName", func(s string) bool { return validateSharedFolderName(s) == nil }, "Shared_2026.v1"},
		{"validateVmName", func(s string) bool { return validateVmName(s) == nil }, "kali-linux 2026"},
		{"validateVmName", func(s string) bool { return validateVmName(s) == nil }, "Ubuntu_24.04-LTS"},
		{"validateSnapshotName", func(s string) bool { return validateSnapshotName(s) == nil }, "Before update - 2026"},
		{"isPlausibleForwardingName", isPlausibleForwardingName, "ssh-fwd"},
		{"isPlausibleHostInterface", isPlausibleHostInterface, "Ethernet-2"},
		{"isPlausibleGuestUsername", isPlausibleGuestUsername, "test-user"},
	}

	for _, tc := range cases {
		t.Run(tc.validator+"/"+tc.value, func(t *testing.T) {
			if !tc.accepts(tc.value) {
				t.Errorf("%s rejected the legitimate name %q", tc.validator, tc.value)
			}
		})
	}
}
