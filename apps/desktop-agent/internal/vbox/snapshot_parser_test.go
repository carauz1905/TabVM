package vbox

import "testing"

func TestParseSnapshotsTree(t *testing.T) {
	out := `SnapshotName="Base"
SnapshotUUID="11111111-1111-1111-1111-111111111111"
SnapshotDescription="clean install"
SnapshotName-1="After updates"
SnapshotUUID-1="22222222-2222-2222-2222-222222222222"
SnapshotName-1-1="Before exploit"
SnapshotUUID-1-1="33333333-3333-3333-3333-333333333333"
CurrentSnapshotName="Before exploit"
CurrentSnapshotUUID="33333333-3333-3333-3333-333333333333"
CurrentSnapshotNode="SnapshotName-1-1"`

	snaps, current := parseSnapshots(out)

	if current != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("current = %q", current)
	}
	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snaps))
	}

	if snaps[0].Name != "Base" || snaps[0].Depth != 0 || snaps[0].Description != "clean install" {
		t.Fatalf("root snapshot wrong: %+v", snaps[0])
	}
	if snaps[1].Name != "After updates" || snaps[1].Depth != 1 {
		t.Fatalf("child snapshot wrong: %+v", snaps[1])
	}
	if snaps[2].Name != "Before exploit" || snaps[2].Depth != 2 {
		t.Fatalf("grandchild snapshot wrong: %+v", snaps[2])
	}
	if snaps[0].Current || snaps[1].Current || !snaps[2].Current {
		t.Fatalf("current flag misassigned: %v %v %v", snaps[0].Current, snaps[1].Current, snaps[2].Current)
	}
}

func TestParseSnapshotsEmpty(t *testing.T) {
	snaps, current := parseSnapshots("")
	if len(snaps) != 0 {
		t.Fatalf("expected no snapshots, got %d", len(snaps))
	}
	if current != "" {
		t.Fatalf("expected no current, got %q", current)
	}
}

func TestValidateSnapshotName(t *testing.T) {
	if err := validateSnapshotName("Before update - 2026"); err != nil {
		t.Fatalf("expected a normal name to pass: %v", err)
	}
	if err := validateSnapshotName(""); err == nil {
		t.Fatal("expected empty name to fail")
	}
	if err := validateSnapshotName("-x"); err == nil {
		t.Fatal("expected leading dash to fail")
	}
	if err := validateSnapshotName("bad\x01name"); err == nil {
		t.Fatal("expected control character to fail")
	}
}
