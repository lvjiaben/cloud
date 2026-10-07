// Package migration applies the local compliance gateway's embedded PostgreSQL schema.
package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
)

// advisoryLockID serializes migration application inside one PostgreSQL database.
const advisoryLockID int64 = 731004620260813

// migrationFiles contains the complete installation-local schema ledger.
//
//go:embed sql/*.sql
var migrationFiles embed.FS

// migration is one immutable embedded script and its ledger checksum.
type migration struct {
	Name     string
	SQL      string
	Checksum string
}

// Apply executes every embedded migration exactly once under a transaction-scoped advisory lock.
func Apply(ctx context.Context, database *sql.DB) (err error) {
	if ctx == nil || database == nil {
		return fmt.Errorf("context and database are required")
	}
	migrations, err := load()
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin gateway migration transaction: %w", err)
	}
	defer finishMigrationTransaction(transaction, &err)
	if _, err = transaction.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockID); err != nil {
		return fmt.Errorf("lock gateway migrations: %w", err)
	}
	if _, err = transaction.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS idc_gateway_schema_migration (
    migration_name VARCHAR(128) PRIMARY KEY,
    checksum_sha256 CHAR(64) NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_migration_checksum CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$')
)`); err != nil {
		return fmt.Errorf("create gateway migration ledger: %w", err)
	}
	for _, item := range migrations {
		var existingChecksum string
		err = transaction.QueryRowContext(ctx,
			`SELECT checksum_sha256 FROM idc_gateway_schema_migration WHERE migration_name = $1`,
			item.Name,
		).Scan(&existingChecksum)
		switch {
		case err == nil:
			if existingChecksum != item.Checksum {
				return fmt.Errorf("gateway migration %s checksum changed", item.Name)
			}
			continue
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("query gateway migration %s: %w", item.Name, err)
		}
		if _, err = transaction.ExecContext(ctx, item.SQL); err != nil {
			return fmt.Errorf("apply gateway migration %s: %w", item.Name, err)
		}
		if _, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_schema_migration (migration_name, checksum_sha256)
VALUES ($1, $2)`, item.Name, item.Checksum); err != nil {
			return fmt.Errorf("record gateway migration %s: %w", item.Name, err)
		}
	}
	if err = transaction.Commit(); err != nil {
		return fmt.Errorf("commit gateway migrations: %w", err)
	}
	return nil
}

// Verify confirms that the installed ledger exactly matches every embedded migration.
func Verify(ctx context.Context, database *sql.DB) (returnError error) {
	if ctx == nil || database == nil {
		return fmt.Errorf("context and database are required")
	}
	migrations, err := load()
	if err != nil {
		return err
	}
	rows, err := database.QueryContext(ctx, `
SELECT migration_name, checksum_sha256
FROM idc_gateway_schema_migration
ORDER BY migration_name`)
	if err != nil {
		return fmt.Errorf("query gateway migration ledger: %w", err)
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			returnError = fmt.Errorf("close gateway migration ledger: %w", closeError)
		}
	}()
	installed := make(map[string]string, len(migrations))
	for rows.Next() {
		var name, checksum string
		if err = rows.Scan(&name, &checksum); err != nil {
			return fmt.Errorf("scan gateway migration ledger: %w", err)
		}
		installed[name] = checksum
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("read gateway migration ledger: %w", err)
	}
	if len(installed) != len(migrations) {
		return fmt.Errorf("gateway migration ledger count is %d, want %d", len(installed), len(migrations))
	}
	for _, item := range migrations {
		if installed[item.Name] != item.Checksum {
			return fmt.Errorf("gateway migration %s is missing or has an unexpected checksum", item.Name)
		}
	}
	return nil
}

// finishMigrationTransaction rolls back unfinished work without masking an earlier error.
func finishMigrationTransaction(transaction *sql.Tx, returnError *error) {
	rollbackError := transaction.Rollback()
	if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) && *returnError == nil {
		*returnError = fmt.Errorf("rollback local gateway migration: %w", rollbackError)
	}
}

// load returns all embedded migrations in deterministic filename order.
func load() ([]migration, error) {
	paths, err := fs.Glob(migrationFiles, "sql/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list embedded gateway migrations: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no embedded gateway migrations found")
	}
	sort.Strings(paths)
	result := make([]migration, 0, len(paths))
	for _, path := range paths {
		content, readErr := migrationFiles.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read embedded gateway migration %s: %w", path, readErr)
		}
		if len(content) == 0 {
			return nil, fmt.Errorf("embedded gateway migration %s is empty", path)
		}
		digest := sha256.Sum256(content)
		result = append(result, migration{
			Name:     path,
			SQL:      string(content),
			Checksum: hex.EncodeToString(digest[:]),
		})
	}
	return result, nil
}
