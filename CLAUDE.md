# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Start PostgreSQL (required before serving)
make db

# Run the server
make serve          # go run ./cmd/server
make dev            # live reload with air, falls back to go run

# Build binaries to bin/
make build

# Tests
go test ./...                                    # all packages
go test ./internal/service/... -v -run TestName  # single test
go test ./... -race -count=1                     # with race detector

# Vet and tidy
go vet ./...
go mod tidy

# OpenAPI spec
make spec-validate  # validate definition/api.yaml
make spec-generate  # regenerate internal/api/api.gen.go from spec
```

**Requirements:** Go 1.22+, FFmpeg + FFprobe in PATH, Docker (for PostgreSQL).

The server runs at `:8080` (HTTP) and `:1935` (RTMP). Migrations run automatically on startup.

## Config

Env vars are read from `.env` then the process environment (process env wins). Copy `.env.example` to `.env` to get started. The server **refuses to start** if `JWT_SECRET` is the default value.

Required vars: `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `OAUTH_REDIRECT_URL`, `SESSION_COOKIE_SECRET`.

Optional vars: `DATABASE_URL`, `DATA_DIR`, `WORKER_COUNT`, `CORS_ORIGINS`, `GOLIVE_WHITELIST`.

## Architecture

### OpenAPI convention

The API is defined in `definition/api.yaml` (OpenAPI 3.0.3, single file). Code generation via `oapi-codegen` produces `internal/api/api.gen.go` which contains:
- All request/response model types
- `ServerInterface` — 30-method interface implemented by `*server.Server`
- `HandlerFromMux(si ServerInterface, r chi.Router)` — registers all routes on a chi router

Adding a new endpoint: update `definition/api.yaml` → `make spec-generate` → implement the new method on `*server.Server`.

### Request lifecycle

All wiring happens in `cmd/server/main.go`: config → DB → repos → services → `server.New(params)` → chi router → `api.HandlerFromMux`.

Router: `go-chi/chi/v5`. Global middleware stack (applied to all routes):
- **CORS** (`go-chi/cors`) — configured via `CORS_ORIGINS`
- **Request ID** (`middleware.RequestIDMiddleware`)
- **Prometheus metrics** (`middleware.MetricsMiddleware`)
- **Security headers** (`securityHeadersMiddleware`)
- **Optional user** (`userAuthMW.OptionalUser`) — populates user context from session cookie

Routes NOT in the OpenAPI spec (registered manually on chi):
- `GET /auth/google/login`, `GET /auth/google/callback` — OAuth redirect flow
- `GET /metrics` — Prometheus scrape endpoint (requires login)
- `GET /thumbnails/*`, `GET /videos/*`, `GET /live/*` — static file serving

Auth checks are applied inside handlers via `middleware.CurrentUser(ctx)`.

### VOD pipeline

```
PUT /chunks/{n}  →  UploadService.ReceiveChunk()
                 →  last chunk triggers assemble() goroutine (2h timeout)
                 →  raw file assembled → TranscodeJob created → jobCh
                 →  TranscodeWorker.run() → TranscodeService.Process()
                 →  Probe → BuildLadder → Encode×N → Segment×N → WriteManifest
                 →  video.status = "ready", HLS in data/hls/{video_id}/
```

`video_id` == `upload_id` (same hex ID throughout). Job channel is an in-memory `chan string`; queued jobs are re-enqueued from the DB on startup.

### Live pipeline

```
RTMP :1935  →  RTMPHandler (go-rtmp) validates stream key
            →  Manager.StartStream() spawns transcodeSession
            →  transcodeSession pipes FLV-wrapped bytes to FFmpeg stdin
            →  FFmpeg writes 2s MPEG-TS segments to data/live/{stream_id}/
            →  on disconnect → Manager.EndStream() → convertToVOD()
```

`EndAllStreams()` is called on SIGTERM so FFmpeg writes `#EXT-X-ENDLIST`.

### JWT auth

`SessionService` issues HS256 tokens with claims `{sid, vid, stid}`. Tokens are passed as `?token=` query param (not headers) because Safari's native HLS player doesn't forward custom headers for sub-playlist requests. The middleware validates the token and enforces that the claimed `vid`/`stid` matches the requested path segment.

### Database

Migrations live in `internal/database/migrations/` (goose `.sql` files embedded in the binary). `database.Migrate(db)` runs `goose.Up()` on startup — a no-op if schema is current. Use `make migrate-new name=<name>` to scaffold a new migration.

`VideoRepo.List()` uses `LEFT JOIN playback_sessions GROUP BY v.id` (not a correlated subquery) to avoid N+1. `VideoRepo.Delete()` cascades manually in a transaction (events → sessions → jobs → video) since there are no `ON DELETE CASCADE` constraints.

## Key Patterns

- **Error handling in handlers:** log with `slog.Error`, return generic `"internal error"` to the client — never `err.Error()` directly in responses. Use `writeError(w, msg, status)` helper.
- **Response helpers:** `writeJSON(w, v, status)`, `writeError(w, msg, status)`, `decodeJSON(r, dst)` defined in `internal/server/errors.go`.
- **Data dirs** are created at `0o700` (private to server process).
- **`os.RemoveAll` failures** are logged at `WARN`, not swallowed silently.
- **Config struct tags** use `github.com/caarlos0/env/v11` — add new env vars by adding a field with `env:"VAR_NAME" envDefault:"..."` tags.
- **Handler files** in `internal/server/` follow `{domain}.go` naming (e.g. `video.go`, `upload.go`). Each file holds all handlers for that domain.
