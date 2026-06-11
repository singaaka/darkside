package store

import (
	"database/sql"

	"github.com/singaaka/darkside-fleet/internal/db/dbgen"
)

type Store struct {
	*dbgen.Queries
	DB *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{Queries: dbgen.New(db), DB: db}
}
