package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func Connect(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	current, err := goose.GetDBVersion(db)
	if err != nil {
		// First run — goose_db_version table doesn't exist yet.
		current = 0
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	next, _ := goose.GetDBVersion(db)
	if next > current {
		slog.Info("database migrated", "from", current, "to", next)
	} else {
		slog.Info("database schema up to date", "version", current)
	}
	return nil
}
