package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenInMemory_MigrationsRun(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error opening in-memory store: %v", err)
	}
	defer s.Close()

	if s.SchemaVersion() != schemaVersion() {
		t.Fatalf("expected schema version %d, got %d", schemaVersion(), s.SchemaVersion())
	}

	status := s.Status()
	if status["configured"] != true || status["available"] != true {
		t.Fatalf("expected configured and available status, got %+v", status)
	}
}

func TestOpen_DefaultPathUsesDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	ctx := context.Background()
	s, err := Open(ctx, Config{})
	if err != nil {
		t.Fatalf("unexpected error opening store: %v", err)
	}
	defer s.Close()

	expected := filepath.Join(dir, "TabVM", "tabvm.db")
	if s.Path() != expected {
		t.Fatalf("expected path %q, got %q", expected, s.Path())
	}
}

func TestOpen_ExplicitDBPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "custom.db")

	ctx := context.Background()
	s, err := Open(ctx, Config{DBPath: dbPath})
	if err != nil {
		t.Fatalf("unexpected error opening store: %v", err)
	}
	defer s.Close()

	if s.Path() != dbPath {
		t.Fatalf("expected path %q, got %q", dbPath, s.Path())
	}
}

func TestOpen_DataDirCreatesParent(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "nested", "tabvm-data")

	ctx := context.Background()
	s, err := Open(ctx, Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("unexpected error opening store: %q", err)
	}
	defer s.Close()

	if s.Path() != filepath.Join(dataDir, "tabvm.db") {
		t.Fatalf("expected db inside data dir, got %q", s.Path())
	}

	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("data dir was not created: %v", err)
	}
}

func TestOpen_RejectsTraversalPath(t *testing.T) {
	ctx := context.Background()
	cases := []string{
		`C:\TabVM\..\data.db`,
		`..\data.db`,
	}

	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			_, err := Open(ctx, Config{DBPath: path})
			if err == nil {
				t.Fatal("expected error for traversal path")
			}
		})
	}
}

func TestVmConsolePort_PersistAndRetrieve(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	port := VmConsolePort{
		VMID:     "11111111-1111-1111-1111-111111111111",
		Port:     5432,
		Address:  "127.0.0.1",
		Protocol: "rdp",
		Source:   "virtualbox-vrde",
	}

	if err := s.SetVmConsolePort(ctx, port); err != nil {
		t.Fatalf("unexpected error setting port: %v", err)
	}

	got, err := s.GetVmConsolePort(ctx, port.VMID)
	if err != nil {
		t.Fatalf("unexpected error getting port: %v", err)
	}
	if got == nil {
		t.Fatal("expected persisted port, got nil")
	}
	if got.Port != port.Port {
		t.Fatalf("expected port %d, got %d", port.Port, got.Port)
	}
	if got.Address != port.Address || got.Protocol != port.Protocol || got.Source != port.Source {
		t.Fatalf("unexpected port metadata: %+v", got)
	}
}

func TestVmConsolePort_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	vmID := "11111111-1111-1111-1111-111111111111"
	if err := s.SetVmConsolePort(ctx, VmConsolePort{VMID: vmID, Port: 5000}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.SetVmConsolePort(ctx, VmConsolePort{VMID: vmID, Port: 5001}); err != nil {
		t.Fatalf("unexpected error updating port: %v", err)
	}

	got, err := s.GetVmConsolePort(ctx, vmID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Port != 5001 {
		t.Fatalf("expected updated port 5001, got %d", got.Port)
	}
}

func TestVmConsolePort_MissingReturnsNil(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	got, err := s.GetVmConsolePort(ctx, "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing port, got %+v", got)
	}
}

func TestVmConsolePort_RequiresVmID(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	if err := s.SetVmConsolePort(ctx, VmConsolePort{Port: 5432}); err == nil {
		t.Fatal("expected error for empty vm_id")
	}
	if _, err := s.GetVmConsolePort(ctx, ""); err == nil {
		t.Fatal("expected error for empty vm_id")
	}
}

func TestAppSetting_PersistAndRetrieve(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	// Allow a temporary key so the persist/retrieve path can be exercised
	// without depending on a specific product setting.
	const key = "test.example"
	AllowedAppSettingKeys[key] = true
	defer delete(AllowedAppSettingKeys, key)

	if err := s.SetAppSetting(ctx, key, "value-1"); err != nil {
		t.Fatalf("unexpected error setting value: %v", err)
	}

	value, err := s.GetAppSetting(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error getting value: %v", err)
	}
	if value != "value-1" {
		t.Fatalf("expected value-1, got %q", value)
	}
}

func TestAppSetting_RejectsDisallowedKey(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	if err := s.SetAppSetting(ctx, "session_token", "secret"); err == nil {
		t.Fatal("expected error for disallowed app setting key")
	}

	value, err := s.GetAppSetting(ctx, "session_token")
	if err != nil {
		t.Fatalf("unexpected error getting value: %v", err)
	}
	if value != "" {
		t.Fatalf("expected disallowed key to remain empty, got %q", value)
	}
}

func TestAppSetting_MissingReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	value, err := s.GetAppSetting(ctx, "nonexistent.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "" {
		t.Fatalf("expected empty value, got %q", value)
	}
}

func TestListOperations_NewestFirst(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	if err := s.LogOperation(ctx, "vm-1", "start", true, "started"); err != nil {
		t.Fatalf("log: %v", err)
	}
	if err := s.LogOperation(ctx, "vm-1", "stop", false, "failed"); err != nil {
		t.Fatalf("log: %v", err)
	}

	entries, err := s.ListOperations(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Newest first: the stop failure was logged last.
	if entries[0].Action != "stop" || entries[0].Success {
		t.Fatalf("expected newest entry to be the failed stop, got %+v", entries[0])
	}
	if entries[1].Action != "start" {
		t.Fatalf("expected oldest entry to be start, got %+v", entries[1])
	}
}

func TestLogOperation_Bounded(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	for i := 0; i < 1005; i++ {
		if err := s.LogOperation(ctx, "vm", "start", true, "ok"); err != nil {
			t.Fatalf("unexpected error logging operation %d: %v", i, err)
		}
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM operation_log`).Scan(&count); err != nil {
		t.Fatalf("unexpected error counting operations: %v", err)
	}
	if count != 1000 {
		t.Fatalf("expected operation log trimmed to 1000, got %d", count)
	}
}

func TestLogOperation_SanitizesPathsAndTokens(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	message := `Failed to copy C:\Users\Admin\secret.txt for vm abcdef0123456789abcdef0123456789`
	if err := s.LogOperation(ctx, "vm", "copy", false, message); err != nil {
		t.Fatalf("unexpected error logging operation: %v", err)
	}

	var stored string
	if err := s.db.QueryRowContext(ctx, `SELECT message FROM operation_log ORDER BY id DESC LIMIT 1`).Scan(&stored); err != nil {
		t.Fatalf("unexpected error reading operation log: %v", err)
	}

	if strings.Contains(stored, `C:\Users\Admin\secret.txt`) {
		t.Fatalf("expected path to be redacted, got %q", stored)
	}
	if strings.Contains(stored, "abcdef0123456789abcdef0123456789") {
		t.Fatalf("expected hex token to be redacted, got %q", stored)
	}
	if !strings.Contains(stored, "[path]") {
		t.Fatalf("expected [path] placeholder, got %q", stored)
	}
	if !strings.Contains(stored, "[redacted]") {
		t.Fatalf("expected [redacted] placeholder, got %q", stored)
	}
}

func TestLogOperation_CapsMessageLength(t *testing.T) {
	ctx := context.Background()
	s, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Close()

	longMessage := strings.Repeat("x ", 400)
	if err := s.LogOperation(ctx, "vm", "start", true, longMessage); err != nil {
		t.Fatalf("unexpected error logging operation: %v", err)
	}

	var stored string
	if err := s.db.QueryRowContext(ctx, `SELECT message FROM operation_log ORDER BY id DESC LIMIT 1`).Scan(&stored); err != nil {
		t.Fatalf("unexpected error reading operation log: %v", err)
	}

	if len(stored) > maxOperationMessage+3 {
		t.Fatalf("expected message to be capped, got length %d", len(stored))
	}
	if !strings.HasSuffix(stored, "...") {
		t.Fatalf("expected capped message to end with ellipsis, got %q", stored)
	}
}

func TestStore_StatusNilStore(t *testing.T) {
	var s *Store
	status := s.Status()
	if status["configured"] != false || status["available"] != false {
		t.Fatalf("expected nil store to report unavailable, got %+v", status)
	}
}

func TestStore_RequiresOpenForOperations(t *testing.T) {
	var s *Store
	ctx := context.Background()

	if err := s.SetVmConsolePort(ctx, VmConsolePort{VMID: "id", Port: 1}); !errors.Is(err, nil) {
		// nil receiver returns a non-nil error via pointer method.
		if err == nil {
			t.Fatal("expected error from nil store")
		}
		if !strings.Contains(err.Error(), "store is not open") {
			t.Fatalf("expected 'store is not open' error, got %v", err)
		}
	}
}
