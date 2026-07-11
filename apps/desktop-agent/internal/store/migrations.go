package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migration is a single schema/version change applied in order by version.
type migration struct {
	version int
	sql     string
}

// migrations contains all schema changes from version 1 upward. New changes
// are appended so existing databases can be upgraded incrementally.
var migrations = []migration{
	{
		version: 1,
		sql: `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS vm_console_ports (
	vm_id TEXT PRIMARY KEY,
	port INTEGER NOT NULL,
	address TEXT NOT NULL DEFAULT '127.0.0.1',
	protocol TEXT NOT NULL DEFAULT 'rdp',
	source TEXT NOT NULL DEFAULT 'virtualbox-vrde',
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS operation_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	vm_id TEXT,
	action TEXT NOT NULL,
	success INTEGER NOT NULL DEFAULT 0,
	message TEXT,
	recorded_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_operation_log_vm_id ON operation_log(vm_id);
CREATE INDEX IF NOT EXISTS idx_operation_log_recorded_at ON operation_log(recorded_at);
`,
	},
}

// migrate applies pending migrations and records each version in
// schema_migrations. It returns the target schema version.
func migrate(ctx context.Context, db *sql.DB) (int, error) {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return 0, fmt.Errorf("create migrations table: %w", err)
	}

	current, err := currentVersion(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("read current schema version: %w", err)
	}

	target := current
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := runMigration(ctx, db, m); err != nil {
			return target, fmt.Errorf("migration %d failed: %w", m.version, err)
		}
		target = m.version
	}

	return target, nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
)
`)
	return err
}

func currentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func runMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		m.version,
		nowUTC().Format(timeFormat),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// schemaVersion returns the latest migration version defined in this binary,
// regardless of whether it has been applied.
func schemaVersion() int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].version
}
