package session

import "database/sql"

// Repo is the PostgreSQL implementation of Repository.
type Repo struct {
	db *sql.DB
}

// New creates a session Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}
