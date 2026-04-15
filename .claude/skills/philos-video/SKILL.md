# PramTube — Codebase Context Skill

## Overview
Self-hosted Go video streaming platform (Go 1.22). Supports VOD upload/transcode, JWT-protected HLS delivery, RTMP live ingest, Google OAuth, and Prometheus metrics.

**Module:** `philos-video`
**Working dir:** `/Users/pram/Workspace/@opam22/philos-video`
**Primary entry point:** `cmd/server/main.go` (HTTP :8080, RTMP :1935)

---

## Architecture

### HTTP Server
- **Router:** `go-chi/chi/v5` — all API routes registered via `api.HandlerFromMux`
- **Generated code:** `gen/api/api.gen.go` (import `philos-video/gen/api`) — types, `ServerInterface`, `HandlerFromMux`
- **Handlers:** `internal/server/{domain}_{MethodName}.go` — one file per operation, all implement `*server.Server`
- **Middleware stack (global):** CORS → RequestID → Prometheus → SecurityHeaders → OptionalUser

### Key Packages
| Package | Purpose |
|---------|---------|
| `cmd/server` | Wires config → DB → repos → services → chi router |
| `gen/api` | Generated types + `ServerInterface` + `HandlerFromMux` (do not edit) |
| `internal/server` | Handler implementations — one file per method, `{domain}_{Method}.go` |
| `internal/server/response_converters.go` | `toResponse*()` — model structs → `api.Response*` types |
| `internal/config` | Env-var config via `caarlos0/env`; server refuses to start with insecure defaults |
| `internal/database` | `sql.Open` + goose migrations (embedded SQL, run on startup) |
| `internal/models` | DB-facing structs + status string constants (`VideoStatusReady`, etc.) |
| `internal/repository` | SQL queries — one file per DB table |
| `internal/service` | Business logic — no HTTP, no SQL |
| `internal/middleware` | JWT HLS auth, user session middleware, Prometheus metrics middleware |
| `internal/live` | RTMP ingest → per-stream FFmpeg session management (`Manager`) |
| `internal/transcoder` | FFmpeg/FFprobe wrappers: probe, encode, segment, manifest |
| `internal/worker` | Goroutine pool draining a `chan string` job channel |

---

## OpenAPI Spec

**Source:** `definition/src/` (split YAML)
- `src/main.yaml` — entry point; all paths use `$ref` to path-item files
- `src/paths/` — one file per URL path, all HTTP methods inside
- `src/components/schemas/` — one file per schema; `Response*` schemas are the canonical API types

**Build pipeline:**
```
definition/src/main.yaml
  → make spec-generate (bundles via swagger-cli + runs oapi-codegen)
  → definition/api.yaml          (committed bundled spec)
  → gen/api/api.gen.go           (generated Go types + routing)
```

**Commands:**
```bash
make spec-validate   # bundle src/ + validate api.yaml
make spec-generate   # bundle + regenerate gen/api/api.gen.go
make spec-docs       # Swagger UI at http://localhost:8081 (Docker)
```

---

## Data Flow — VOD Upload
```
PUT /api/v1/uploads/{id}/chunks/{n}
  → UploadService.ReceiveChunk()
  → last chunk triggers assemble() goroutine (2h timeout)
  → TranscodeJob queued on in-memory chan string
  → TranscodeWorker → TranscodeService.Process()
  → FFprobe → BuildLadder → Encode×N → Segment×N → WriteManifest
  → video.status = "ready", HLS in data/hls/{video_id}/
```

## Data Flow — Live Streaming
```
OBS/FFmpeg → RTMP :1935/live/{stream_key}
  → RTMPHandler validates stream key → Manager.StartStream()
  → transcodeSession pipes FLV-wrapped bytes to FFmpeg stdin
  → FFmpeg writes 2s TS segments to data/live/{stream_id}/
  → on disconnect → Manager.EndStream() → convertToVOD()
```

---

## Response Type Conventions

- Handler methods return `api.Response*` types (e.g. `api.ResponseVideo`, `api.ResponseVideoStatus`)
- Conversions from model structs go in `internal/server/response_converters.go`
- Inline enum fields in the spec generate typed aliases — always cast: `api.VideoStatusEnum(v.Status)`

---

## Key Patterns

- **Error handling:** `slog.Error(...)` then `writeError(w, "internal error", status)` — never return `err.Error()` to the client
- **Auth in handlers:** `middleware.CurrentUser(ctx)` returns `(*models.User, bool)` — check the bool, don't panic
- **Path params:** oapi-codegen passes them as direct function args — never use `chi.URLParam(r, "id")`
- **Config:** add new env vars as struct fields with `env:"VAR_NAME" envDefault:"..."` tags on `internal/config/config.go`
- **Migrations:** `make migrate-new name=<name>` scaffolds a goose SQL file in `internal/database/migrations/`

---

## Environment Variables (key ones)
| Variable | Notes |
|----------|-------|
| `JWT_SECRET` | Required — server exits if using the code default |
| `DATABASE_URL` | Default: `postgres://philos:philos@localhost:5433/philos_video?sslmode=disable` |
| `DATA_DIR` | Default: `./data` — root for chunks/raw/hls/live |
| `WORKER_COUNT` | Default: `2` — parallel transcode goroutines |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Required for OAuth |
| `SESSION_COOKIE_SECRET` | Required — min 32 chars |
| `GOLIVE_WHITELIST` | Comma-separated emails allowed to create stream keys |

---

## Known Limitations
1. Safari native HLS won't work with JWT tokens (use hls.js in browser)
2. No JWT refresh — set `JWT_EXPIRY` longer than the longest expected session
3. Live streams require audio; video-only RTMP streams fail FFmpeg
4. Server restart kills active live streams — run `UPDATE live_streams SET status='ended' WHERE status='live'` after crash
