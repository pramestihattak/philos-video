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
```

**Requirements:** Go 1.22+, FFmpeg + FFprobe in PATH, Docker (for PostgreSQL).

The server runs at `:8080` (HTTP) and `:1935` (RTMP). Migrations run automatically on startup.

## Config

Env vars are read from `.env` then the process environment (process env wins). Copy `.env.example` to `.env` to get started. The server **refuses to start** if `JWT_SECRET` is the default value.

Key vars: `JWT_SECRET` (required), `GO_LIVE_PIN` (optional, warns if unset), `DATABASE_URL`, `DATA_DIR`, `WORKER_COUNT`.

## Architecture

### Request lifecycle

All wiring happens in `cmd/server/main.go`: config → DB → repos → services → handlers → mux. There is no framework — routes use Go 1.22 method+pattern syntax (`"POST /api/v1/uploads"`).

Middleware stack (applied per-route, not globally):
- **IP rate limiter** (`middleware.NewIPRateLimiter`) — on upload init and session creation
- **JWT auth** (`AuthMiddleware.RequirePlaybackToken` / `RequireLiveToken`) — on all HLS file serving under `/videos/` and `/live/`
- **GoLive PIN gate** (`GoLivePinGate` / `GoLivePinAPIGate`) — on `/go-live` page and stream-key API

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

### QoE aggregator

`qoe.Aggregator` maintains an in-memory sliding window (5 min) of `PlaybackEvent` structs. It recalculates metrics every second and broadcasts to SSE subscribers via a fan-out channel. The `videoRepo` pointer is nullable — pass `nil` in tests to avoid DB calls.

### Database

Schema is inlined in `internal/database/postgres.go` as `migrationSQL` — all `CREATE IF NOT EXISTS` / `ALTER IF NOT EXISTS`, so it's safe to re-run. No migration library.

`VideoRepo.List()` uses `LEFT JOIN playback_sessions GROUP BY v.id` (not a correlated subquery) to avoid N+1. `VideoRepo.Delete()` cascades manually in a transaction (events → sessions → jobs → video) since there are no `ON DELETE CASCADE` constraints.

## Key Patterns

- **Error handling in handlers:** log with `slog.Error`, return generic `"internal error"` to the client — never `err.Error()` directly in `http.Error()`.
- **Data dirs** are created at `0o700` (private to server process).
- **`os.RemoveAll` failures** are logged at `WARN`, not swallowed silently.
- **Config struct tags** use `github.com/caarlos0/env/v11` — add new env vars by adding a field with `env:"VAR_NAME" envDefault:"..."` tags.
