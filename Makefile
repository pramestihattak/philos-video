.PHONY: transcode serve dev build clean db stop help

INPUT ?= video.mp4
OUTPUT ?= ./output

## db: start postgres (docker compose)
db:
	docker compose up -d postgres

## stop: stop postgres
stop:
	docker compose down

## serve: start the platform server (reads env: PORT, DATABASE_URL, DATA_DIR, WORKER_COUNT)
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

## clean: remove build artifacts and data directory
clean:
	rm -rf bin/ data/

help:
	@grep -E '^##' Makefile | sed 's/## //'
