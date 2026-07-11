package vbox

import (
	"testing"
)

func TestParseRunningVmIDs(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "empty output",
			output:   "",
			expected: nil,
		},
		{
			name:     "two running vms",
			output:   "\"VM One\" {11111111-1111-1111-1111-111111111111}\r\n\"VM Two\" {22222222-2222-2222-2222-222222222222}",
			expected: []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"},
		},
		{
			name:     "ignores malformed lines",
			output:   "not a vm\r\n\"Good VM\" {33333333-3333-3333-3333-333333333333}\r\nmalformed",
			expected: []string{"33333333-3333-3333-3333-333333333333"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := parseRunningVmIDs(tt.output)
			if len(ids) != len(tt.expected) {
				t.Fatalf("expected %d IDs, got %d", len(tt.expected), len(ids))
			}
			for i, expected := range tt.expected {
				if ids[i] != expected {
					t.Fatalf("ID %d mismatch: got %q, want %q", i, ids[i], expected)
				}
			}
		})
	}
}

func TestNormalizeVmState(t *testing.T) {
	cases := map[string]string{
		"running":  "running",
		"starting": "booting",
		"paused":   "paused",
		"saved":    "saved",
		"poweroff": "powered off",
		"aborted":  "aborted",
		"":         "",
		"WeirdNew": "weirdnew", // unknown states pass through lowercased
	}
	for raw, want := range cases {
		if got := normalizeVmState(raw); got != want {
			t.Errorf("normalizeVmState(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseVmState(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "running",
			output:   "name=\"VM\"\nVMState=\"running\"",
			expected: "running",
		},
		{
			name:     "powered off",
			output:   "name=\"VM\"\nVMState=\"poweroff\"",
			expected: "poweroff",
		},
		{
			name:     "no state",
			output:   "name=\"VM\"",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := parseVmState(tt.output)
			if state != tt.expected {
				t.Fatalf("expected state %q, got %q", tt.expected, state)
			}
		})
	}
}

func TestParseVRDEInfo(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected vrdeInfo
	}{
		{
			name:     "empty output",
			output:   "",
			expected: vrdeInfo{},
		},
		{
			name:   "enabled with local address",
			output: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"5001\"",
			expected: vrdeInfo{
				enabled: true,
				address: "127.0.0.1",
				port:    "5001",
			},
		},
		{
			name:   "disabled",
			output: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\"",
			expected: vrdeInfo{
				enabled: false,
				address: "",
				port:    "0",
			},
		},
		{
			name:   "mixed case keys",
			output: "VRDE=\"on\"\nVRDEAddress=\"127.0.0.1\"\nVRDEPort=\"3389\"",
			expected: vrdeInfo{
				enabled: true,
				address: "127.0.0.1",
				port:    "3389",
			},
		},
		{
			name:   "ignores unrelated lines",
			output: "name=\"VM\"\nvrde=\"on\"\nvmstate=\"running\"",
			expected: vrdeInfo{
				enabled: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseVRDEInfo(tt.output)
			if info.enabled != tt.expected.enabled {
				t.Fatalf("enabled mismatch: got %v, want %v", info.enabled, tt.expected.enabled)
			}
			if info.address != tt.expected.address {
				t.Fatalf("address mismatch: got %q, want %q", info.address, tt.expected.address)
			}
			if info.port != tt.expected.port {
				t.Fatalf("port mismatch: got %q, want %q", info.port, tt.expected.port)
			}
		})
	}
}

func TestParseListVmsOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []struct {
			id    string
			name  string
			state string
		}
	}{
		{
			name:     "empty output",
			output:   "",
			expected: nil,
		},
		{
			name:   "two vms",
			output: "\"VM One\" {11111111-1111-1111-1111-111111111111}\r\n\"VM Two\" {22222222-2222-2222-2222-222222222222}",
			expected: []struct {
				id    string
				name  string
				state string
			}{
				{id: "11111111-1111-1111-1111-111111111111", name: "VM One", state: "listed"},
				{id: "22222222-2222-2222-2222-222222222222", name: "VM Two", state: "listed"},
			},
		},
		{
			name:   "ignores malformed lines",
			output: "not a vm\r\n\"Good VM\" {33333333-3333-3333-3333-333333333333}\r\nmalformed",
			expected: []struct {
				id    string
				name  string
				state string
			}{
				{id: "33333333-3333-3333-3333-333333333333", name: "Good VM", state: "listed"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vms := parseListVmsOutput(tt.output)
			if len(vms) != len(tt.expected) {
				t.Fatalf("expected %d VMs, got %d", len(tt.expected), len(vms))
			}
			for i, expected := range tt.expected {
				if vms[i].ID != expected.id || vms[i].Name != expected.name || vms[i].State != expected.state {
					t.Fatalf("VM %d mismatch: got %+v, want id=%q name=%q state=%q",
						i, vms[i], expected.id, expected.name, expected.state)
				}
			}
		})
	}
}
