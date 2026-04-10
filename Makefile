.PHONY: transcode serve dev build clean db stop migrate-new migrate-up migrate-down migrate-status help

-include .env
export

MIGRATIONS_DIR := internal/database/migrations
GOOSE         := go run ./cmd/migrate

INPUT ?= video.mp4
OUTPUT ?= ./output

## db: start postgres (docker compose)
db:
	docker compose up -d postgres

## stop: stop postgres
stop:
	docker compose down

## serve: start the platform server (reads env: PORT, DATABASE_URL, DATA_DIR, WORKER_COUNT, RTMP_PORT)
serve:
	go run ./cmd/server

## dev: start server with live reload (requires air); falls back to go run
dev:
	@air 2>/dev/null || go run ./cmd/server

## transcode: encode INPUT into HLS segments under OUTPUT (Phase 1 CLI)
transcode:
	go run ./cmd/transcode -input $(INPUT) -output $(OUTPUT)

## build: compile both binaries to ./bin/
build:
	@mkdir -p bin
	go build -o bin/server    ./cmd/server
	go build -o bin/transcode ./cmd/transcode

## migrate-new name=<name>: create a new SQL migration file
migrate-new:
	@test -n "$(name)" || (echo "Usage: make migrate-new name=<migration_name>" && exit 1)
	@count=$$(ls $(MIGRATIONS_DIR)/*.sql 2>/dev/null | wc -l | tr -d ' \t'); \
	 n=$$(printf "%05d" $$((count + 1))); \
	 f="$(MIGRATIONS_DIR)/$${n}_$(name).sql"; \
	 printf -- "-- +goose Up\n\n\n-- +goose Down\n\n" > "$$f"; \
	 echo "Created $$f"

## migrate-up: apply all pending migrations
migrate-up:
	$(GOOSE) up

## migrate-down: roll back the last applied migration
migrate-down:
	$(GOOSE) down

## migrate-status: show applied/pending migration status
migrate-status:
	$(GOOSE) status

## clean: remove build artifacts and data directory
clean:
	rm -rf bin/ data/

## help: list available targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
