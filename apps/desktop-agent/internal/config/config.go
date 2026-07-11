package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// Agent holds the runtime configuration for the local HTTP agent.
type Agent struct {
	BindAddress            string
	BindPort               int
	SessionToken           string
	Environment            string
	VBoxManagePaths        []string
	DataDir                string
	DBPath                 string
}

// Default VBoxManage search paths on Windows.
var defaultVBoxManagePaths = []string{
	`C:\Program Files\Oracle\VirtualBox\VBoxManage.exe`,
	`C:\Program Files (x86)\Oracle\VirtualBox\VBoxManage.exe`,
}

// Load reads configuration from environment variables.
// It applies sensible defaults for a local development agent.
func Load() (*Agent, error) {
	bindPort, err := parsePort("TABVM_AGENT_BIND_PORT", 5230)
	if err != nil {
		return nil, err
	}

	bindAddress := getEnv("TABVM_AGENT_BIND_ADDRESS", "127.0.0.1")
	if err := validateBindAddress(bindAddress); err != nil {
		return nil, err
	}

	dataDir, err := validatePathEnv("TABVM_DATA_DIR", os.Getenv("TABVM_DATA_DIR"))
	if err != nil {
		return nil, err
	}

	dbPath, err := validatePathEnv("TABVM_DB_PATH", os.Getenv("TABVM_DB_PATH"))
	if err != nil {
		return nil, err
	}

	cfg := &Agent{
		BindAddress:            bindAddress,
		BindPort:               bindPort,
		Environment:            getEnv("TABVM_AGENT_ENV", "Development"),
		VBoxManagePaths:        getVBoxManagePaths(),
		DataDir:                dataDir,
		DBPath:                 dbPath,
	}

	token := getEnv("TABVM_AGENT_SESSION_TOKEN", getEnv("TabVM__Agent__SessionToken", ""))
	cfg.SessionToken = strings.TrimSpace(token)

	return cfg, nil
}

// IsDevelopment returns true when the agent is running in development mode.
func (a *Agent) IsDevelopment() bool {
	return strings.EqualFold(a.Environment, "Development")
}

// ListenAddress returns the host:port string used by the HTTP server.
func (a *Agent) ListenAddress() string {
	return fmt.Sprintf("%s:%d", a.BindAddress, a.BindPort)
}

func validateBindAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("TABVM_AGENT_BIND_ADDRESS cannot be empty")
	}

	ip := net.ParseIP(addr)
	if ip != nil {
		if !ip.IsLoopback() {
			return fmt.Errorf("TABVM_AGENT_BIND_ADDRESS must be a loopback address; %q is not allowed", addr)
		}
		return nil
	}

	if strings.EqualFold(addr, "localhost") {
		return nil
	}

	return fmt.Errorf("TABVM_AGENT_BIND_ADDRESS must be a loopback address (e.g. 127.0.0.1 or localhost); %q is not allowed", addr)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parsePort(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: must be an integer", key, value)
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("invalid %s %d: must be between 1 and 65535", key, n)
	}
	return n, nil
}

func getVBoxManagePaths() []string {
	paths := defaultVBoxManagePaths
	if extra := os.Getenv("TABVM_VBOX_MANAGE_PATHS"); extra != "" {
		parts := strings.Split(extra, string(os.PathListSeparator))
		paths = append(parts, paths...)
	}
	return paths
}

// validatePathEnv validates an environment variable that resolves to a
// filesystem path. Empty values are allowed (meaning "use the default"), but
// values containing path traversal sequences are rejected so startup fails fast
// with a clear error instead of silently ignoring the misconfiguration.
func validatePathEnv(key, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.Contains(value, "..") {
		return "", fmt.Errorf("%s contains path traversal sequences and is not allowed", key)
	}
	return value, nil
}
