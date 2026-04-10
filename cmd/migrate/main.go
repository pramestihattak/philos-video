// cmd/migrate runs goose migrations against DATABASE_URL.
// Usage: go run ./cmd/migrate <up|down|status>
package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
	_ "github.com/lib/pq"
)

const migrationsDir = "internal/database/migrations"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|status>")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is not set")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "set dialect: %v\n", err)
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "up":
		err = goose.Up(db, migrationsDir)
	case "down":
		err = goose.Down(db, migrationsDir)
	case "status":
		err = goose.Status(db, migrationsDir)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s (want: up, down, status)\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "goose %s: %v\n", cmd, err)
		os.Exit(1)
	}
}
