package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate brings the database up to the latest schema version using goose.
// Migration files live in internal/db/migrations and are embedded into the
// binary so the runtime container doesn't need a separate volume mount.
//
// Goose tracks applied versions in a goose_db_version table that it creates
// itself; calling Up on an already-current DB is a no-op.
func Migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetTableName("schema_migrations")
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
