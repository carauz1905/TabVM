package vbox

import (
	"io/fs"
	"testing"
	"time"
)

func TestSanitizeTransferFilename(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain", "report.pdf", "report.pdf", false},
		{"windows path stripped", `C:\evil\..\..\report.pdf`, "report.pdf", false},
		{"posix traversal stripped", "../../etc/passwd", "passwd", false},
		{"nested segments", "a/b/c.txt", "c.txt", false},
		{"invalid chars replaced", `bad:name?.txt`, "bad_name_.txt", false},
		{"spaces kept internally", "  my file.txt  ", "my file.txt", false},
		{"dotdot rejected", "..", "", true},
		{"dot rejected", ".", "", true},
		{"empty rejected", "   ", "", true},
		{"only separators rejected", `\\`, "", true},
		{"trailing dot trimmed (windows)", "name.", "name", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeTransferFilename(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

type fakeFileInfo struct {
	dir bool
}

func (f fakeFileInfo) Name() string       { return "x" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }

func TestChooseWritableShare(t *testing.T) {
	original := statPath
	defer func() { statPath = original }()

	// The first share's host path does not exist; the second is a directory.
	statPath = func(p string) (fs.FileInfo, error) {
		if p == `C:\good` {
			return fakeFileInfo{dir: true}, nil
		}
		return nil, fs.ErrNotExist
	}

	folders := []sharedFolderInfo{
		{name: "missing", hostPath: `C:\missing`},
		{name: "good", hostPath: `C:\good`},
	}
	got, ok := chooseWritableShare(folders)
	if !ok {
		t.Fatal("expected a writable share to be chosen")
	}
	if got.name != "good" {
		t.Fatalf("chose %q, want \"good\"", got.name)
	}

	if _, ok := chooseWritableShare(nil); ok {
		t.Fatal("expected no share for empty input")
	}
}

func TestGuestHomeDir(t *testing.T) {
	if got := guestHomeDir("root"); got != "/root" {
		t.Fatalf("root home = %q, want /root", got)
	}
	if got := guestHomeDir("student"); got != "/home/student" {
		t.Fatalf("student home = %q, want /home/student", got)
	}
}
