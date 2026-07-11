package config

import (
	"os"
	"strings"
	"testing"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BindAddress != "127.0.0.1" {
		t.Errorf("expected bind address 127.0.0.1, got %q", cfg.BindAddress)
	}
	if cfg.BindPort != 5230 {
		t.Errorf("expected bind port 5230, got %d", cfg.BindPort)
	}
	if cfg.Environment != "Development" {
		t.Errorf("expected environment Development, got %q", cfg.Environment)
	}
	if cfg.SessionToken != "" {
		t.Errorf("expected empty session token by default, got %q", cfg.SessionToken)
	}
}

func TestLoad_ValidPort(t *testing.T) {
	setEnv(t, "TABVM_AGENT_BIND_PORT", "8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BindPort != 8080 {
		t.Errorf("expected bind port 8080, got %d", cfg.BindPort)
	}
}

func TestLoad_InvalidPortStringFailsFast(t *testing.T) {
	setEnv(t, "TABVM_AGENT_BIND_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port string")
	}
}

func TestLoad_PortOutOfRangeFailsFast(t *testing.T) {
	cases := []string{"0", "65536", "-1", "99999"}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			setEnv(t, "TABVM_AGENT_BIND_PORT", value)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for port %q", value)
			}
		})
	}
}

func TestLoad_SessionToken(t *testing.T) {
	setEnv(t, "TABVM_AGENT_SESSION_TOKEN", "my-secret-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionToken != "my-secret-token" {
		t.Errorf("expected session token my-secret-token, got %q", cfg.SessionToken)
	}
}

func TestLoad_LegacySessionToken(t *testing.T) {
	setEnv(t, "TabVM__Agent__SessionToken", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionToken != "legacy-token" {
		t.Errorf("expected legacy token, got %q", cfg.SessionToken)
	}
}

func TestLoad_PrefersPrimarySessionToken(t *testing.T) {
	setEnv(t, "TABVM_AGENT_SESSION_TOKEN", "primary-token")
	setEnv(t, "TabVM__Agent__SessionToken", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionToken != "primary-token" {
		t.Errorf("expected primary token, got %q", cfg.SessionToken)
	}
}

func TestLoad_TrimmedSessionToken(t *testing.T) {
	setEnv(t, "TABVM_AGENT_SESSION_TOKEN", "  spaced-token  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionToken != "spaced-token" {
		t.Errorf("expected trimmed token, got %q", cfg.SessionToken)
	}
}

func TestLoad_VBoxManagePaths(t *testing.T) {
	setEnv(t, "TABVM_VBOX_MANAGE_PATHS", `C:\Custom\VBoxManage.exe`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.VBoxManagePaths) == 0 || cfg.VBoxManagePaths[0] != `C:\Custom\VBoxManage.exe` {
		t.Errorf("expected custom path first, got %v", cfg.VBoxManagePaths)
	}
}

func TestListenAddress(t *testing.T) {
	cfg := &Agent{
		BindAddress: "127.0.0.1",
		BindPort:    5230,
	}
	if got := cfg.ListenAddress(); got != "127.0.0.1:5230" {
		t.Errorf("expected listen address 127.0.0.1:5230, got %q", got)
	}
}

func TestIsDevelopment(t *testing.T) {
	setEnv(t, "TABVM_AGENT_ENV", "Production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IsDevelopment() {
		t.Error("expected Production environment not to be development")
	}
}

func TestLoad_RejectsNonLoopbackAddress(t *testing.T) {
	cases := []string{"0.0.0.0", "192.168.1.1", "10.0.0.1", "172.16.0.1", "8.8.8.8"}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			setEnv(t, "TABVM_AGENT_BIND_ADDRESS", value)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for bind address %q", value)
			}
		})
	}
}

func TestLoad_AcceptsLoopbackAddresses(t *testing.T) {
	cases := []string{"127.0.0.1", "::1", "localhost"}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			setEnv(t, "TABVM_AGENT_BIND_ADDRESS", value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.BindAddress != value {
				t.Errorf("expected bind address %q, got %q", value, cfg.BindAddress)
			}
		})
	}
}

func TestLoad_DataDirAndDBPath(t *testing.T) {
	setEnv(t, "TABVM_DATA_DIR", `C:\TabVM\Data`)
	setEnv(t, "TABVM_DB_PATH", `C:\TabVM\Data\custom.db`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DataDir != `C:\TabVM\Data` {
		t.Errorf("expected data dir, got %q", cfg.DataDir)
	}
	if cfg.DBPath != `C:\TabVM\Data\custom.db` {
		t.Errorf("expected db path, got %q", cfg.DBPath)
	}
}

func TestLoad_RejectsInvalidDBPath(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"traversal in db path", "TABVM_DB_PATH", `C:\TabVM\..\data.db`},
		{"traversal in data dir", "TABVM_DATA_DIR", `C:\TabVM\..\Data`},
		{"relative traversal in db path", "TABVM_DB_PATH", `..\data.db`},
		{"relative traversal in data dir", "TABVM_DATA_DIR", `..\Data`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, tc.key, tc.value)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error for invalid path env var")
			}
			if !strings.Contains(err.Error(), "traversal") {
				t.Fatalf("expected traversal error, got %v", err)
			}
		})
	}
}

func TestLoad_ClearsEnvironment(t *testing.T) {
	// Ensure no leaked env vars from other tests affect this one.
	for _, key := range []string{
		"TABVM_AGENT_BIND_PORT",
		"TABVM_AGENT_SESSION_TOKEN",
		"TabVM__Agent__SessionToken",
		"TABVM_AGENT_ENV",
		"TABVM_VBOX_MANAGE_PATHS",
		"TABVM_AGENT_BIND_ADDRESS",
		"TABVM_DATA_DIR",
		"TABVM_DB_PATH",
	} {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BindPort != 5230 {
		t.Errorf("expected default port, got %d", cfg.BindPort)
	}
	if cfg.DataDir != "" {
		t.Errorf("expected empty data dir by default, got %q", cfg.DataDir)
	}
	if cfg.DBPath != "" {
		t.Errorf("expected empty db path by default, got %q", cfg.DBPath)
	}
}
