package store

import (
	"database/sql"

	"github.com/singaaka/darkside/internal/db/dbgen"
)

// Store wraps the sqlc-generated Queries with the underlying *sql.DB so callers
// can both run typed queries and begin transactions through one object.
type Store struct {
	*dbgen.Queries
	DB *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{
		Queries: dbgen.New(db),
		DB:      db,
	}
}
