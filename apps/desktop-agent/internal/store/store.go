package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	driverName          = "sqlite"
	timeFormat          = time.RFC3339
	maxOperationMessage = 500
)

// AllowedAppSettingKeys is the allowlist of non-secret settings that may be
// persisted in app_settings. Keys outside this set are rejected to prevent the
// store from becoming an unrestricted key/value dump. It is intentionally empty
// until a feature needs a persisted non-secret setting; add the specific key
// here when that happens.
var AllowedAppSettingKeys = map[string]bool{}

var (
	// absolutePathPattern matches Windows-style absolute paths such as
	// C:\Program Files\TabVM\file.db. These are redacted before an operation
	// log message is persisted so host filesystem details do not leak into the
	// local audit log.
	absolutePathPattern = regexp.MustCompile(`\b[A-Za-z]:[\\/][^\s]*`)

	// hexTokenPattern matches long hexadecimal strings that are commonly used
	// for tokens, session IDs, or API keys. They are redacted defensively even
	// though the operation log should not contain secrets.
	hexTokenPattern = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
)

// nowUTC is overridable in tests for deterministic timestamps.
var nowUTC = func() time.Time { return time.Now().UTC() }

// Store provides persistence for local state such as VM console port
// assignments, non-secret application settings, and a bounded operation log.
type Store struct {
	db     *sql.DB
	dsn    string
	path   string
	schema int
}

// Config controls where the SQLite database is created.
type Config struct {
	// DBPath is an explicit SQLite database file path. When set, it takes
	// precedence over DataDir.
	DBPath string
	// DataDir is a directory where the default "tabvm.db" file is stored.
	// When empty, the default Windows user-local directory is used.
	DataDir string
}

// Open creates or opens the SQLite database at the configured location and
// applies pending migrations. The returned Store must be closed when the
// agent shuts down.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	path, err := resolveDBPath(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}

	if err := ensureParentDir(path); err != nil {
		return nil, fmt.Errorf("prepare database directory: %w", err)
	}

	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	schema, err := migrate(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &Store{
		db:     db,
		dsn:    dsn,
		path:   path,
		schema: schema,
	}, nil
}

// OpenInMemory opens an in-memory database for tests. It still runs migrations.
func OpenInMemory(ctx context.Context) (*Store, error) {
	db, err := sql.Open(driverName, ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}

	schema, err := migrate(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate in-memory database: %w", err)
	}

	return &Store{
		db:     db,
		dsn:    ":memory:",
		path:   ":memory:",
		schema: schema,
	}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Path returns the resolved database path. For in-memory stores this returns
// ":memory:".
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// SchemaVersion returns the schema version the store was opened at.
func (s *Store) SchemaVersion() int {
	if s == nil {
		return 0
	}
	return s.schema
}

// Status reports whether the database is configured and reachable without
// exposing the underlying filesystem path.
func (s *Store) Status() map[string]any {
	if s == nil || s.db == nil {
		return map[string]any{
			"configured": false,
			"available":  false,
			"schema":     0,
		}
	}

	available := s.db.PingContext(context.Background()) == nil
	return map[string]any{
		"configured": true,
		"available":  available,
		"schema":     s.schema,
	}
}

// SetVmConsolePort persists the assigned console port for a VM.
func (s *Store) SetVmConsolePort(ctx context.Context, port VmConsolePort) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store is not open")
	}
	if port.VMID == "" {
		return fmt.Errorf("vm_id is required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO vm_console_ports (vm_id, port, address, protocol, source, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(vm_id) DO UPDATE SET
	port = excluded.port,
	address = excluded.address,
	protocol = excluded.protocol,
	source = excluded.source,
	updated_at = excluded.updated_at
`, port.VMID, port.Port, port.Address, port.Protocol, port.Source, nowUTC().Format(timeFormat))
	if err != nil {
		return fmt.Errorf("set vm console port: %w", err)
	}
	return nil
}

// GetVmConsolePort returns the persisted console port for a VM, or nil if no
// port has been assigned.
func (s *Store) GetVmConsolePort(ctx context.Context, vmID string) (*VmConsolePort, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store is not open")
	}
	if vmID == "" {
		return nil, fmt.Errorf("vm_id is required")
	}

	row := s.db.QueryRowContext(ctx, `
SELECT vm_id, port, address, protocol, source, updated_at
FROM vm_console_ports
WHERE vm_id = ?
`, vmID)

	var p VmConsolePort
	var updatedAt string
	err := row.Scan(&p.VMID, &p.Port, &p.Address, &p.Protocol, &p.Source, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get vm console port: %w", err)
	}
	p.UpdatedAt, err = time.Parse(timeFormat, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return &p, nil
}

// SetAppSetting persists a non-secret application setting. Only keys in
// AllowedAppSettingKeys are accepted.
func (s *Store) SetAppSetting(ctx context.Context, key, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store is not open")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if !AllowedAppSettingKeys[key] {
		return fmt.Errorf("app setting key %q is not allowed", key)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
	value = excluded.value,
	updated_at = excluded.updated_at
`, key, value, nowUTC().Format(timeFormat))
	if err != nil {
		return fmt.Errorf("set app setting: %w", err)
	}
	return nil
}

// GetAppSetting returns a persisted setting value, or the empty string if it
// does not exist.
func (s *Store) GetAppSetting(ctx context.Context, key string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("store is not open")
	}

	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get app setting: %w", err)
	}
	return value, nil
}

// LogOperation appends a bounded audit entry for a VM lifecycle or console
// action. Only the most recent entries are retained. The message is capped in
// length and sanitized to avoid persisting host paths or secret-like tokens.
func (s *Store) LogOperation(ctx context.Context, vmID, action string, success bool, message string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store is not open")
	}
	if action == "" {
		return fmt.Errorf("action is required")
	}

	message = sanitizeOperationMessage(message)

	_, err := s.db.ExecContext(ctx, `
INSERT INTO operation_log (vm_id, action, success, message, recorded_at)
VALUES (?, ?, ?, ?, ?)
`, vmID, action, success, message, nowUTC().Format(timeFormat))
	if err != nil {
		return fmt.Errorf("log operation: %w", err)
	}

	return s.trimOperationLog(ctx)
}

// sanitizeOperationMessage caps message length and redacts absolute paths and
// long hex-like tokens so the operation log does not retain host filesystem
// details or accidental secrets.
func sanitizeOperationMessage(message string) string {
	message = absolutePathPattern.ReplaceAllString(message, "[path]")
	message = hexTokenPattern.ReplaceAllString(message, "[redacted]")

	if len(message) > maxOperationMessage {
		message = message[:maxOperationMessage] + "..."
	}
	return message
}

// trimOperationLog keeps only the most recent 1000 entries. The limit is
// intentionally generous for a local agent and prevents unbounded growth.
func (s *Store) trimOperationLog(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM operation_log
WHERE id NOT IN (
	SELECT id FROM operation_log ORDER BY recorded_at DESC, id DESC LIMIT 1000
)
`)
	if err != nil {
		return fmt.Errorf("trim operation log: %w", err)
	}
	return nil
}

// ListOperations returns the most recent operation-log entries, newest first.
// limit is clamped to a sane range.
func (s *Store) ListOperations(ctx context.Context, limit int) ([]OperationLogEntry, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store is not open")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT vm_id, action, success, message, recorded_at
FROM operation_log
ORDER BY recorded_at DESC, id DESC
LIMIT ?
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	defer rows.Close()

	entries := make([]OperationLogEntry, 0, limit)
	for rows.Next() {
		var e OperationLogEntry
		var recordedAt string
		if err := rows.Scan(&e.VMID, &e.Action, &e.Success, &e.Message, &recordedAt); err != nil {
			return nil, fmt.Errorf("scan operation: %w", err)
		}
		e.RecordedAt, _ = time.Parse(timeFormat, recordedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// resolveDBPath determines the database file path from configuration or the
// default Windows user-local location. It rejects path traversal but does not
// perform a full security audit of the configured path.
func resolveDBPath(cfg Config) (string, error) {
	if cfg.DBPath != "" {
		path := strings.TrimSpace(cfg.DBPath)
		if err := validatePath(path); err != nil {
			return "", err
		}
		return filepath.Clean(path), nil
	}

	var dir string
	if cfg.DataDir != "" {
		dir = strings.TrimSpace(cfg.DataDir)
		if err := validatePath(dir); err != nil {
			return "", err
		}
		dir = filepath.Clean(dir)
	} else {
		dir = defaultDataDir()
	}

	if dir == "" {
		return "", fmt.Errorf("default data directory is not available on this platform")
	}

	return filepath.Join(dir, "tabvm.db"), nil
}

func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	// Reject obvious path traversal. This is a guard against accidental
	// misconfiguration, not a security boundary.
	if strings.Contains(path, "..") {
		return fmt.Errorf("path cannot contain traversal sequences")
	}
	return nil
}

func ensureParentDir(path string) error {
	if path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0750)
}

func defaultDataDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = os.Getenv("APPDATA")
	}
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, "TabVM")
}
