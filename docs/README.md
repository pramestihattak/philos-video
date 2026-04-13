# philos-video — Developer Documentation

> **Overview & quick start:** [`../README.md`](../README.md)

A self-hosted video streaming **API server** written in Go. The backend exposes a REST API defined by an OpenAPI 3.0 spec; the frontend is a separate application. Supports chunked upload, server-side transcoding to adaptive-bitrate HLS, live RTMP ingest with real-time HLS delivery, Google OAuth authentication, and JWT-secured playback.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Prerequisites](#2-prerequisites)
3. [Quick Start](#3-quick-start)
4. [Configuration](#4-configuration)
5. [Directory Structure](#5-directory-structure)
6. [OpenAPI Workflow](#6-openapi-workflow)
7. [Database Schema](#7-database-schema)
8. [HTTP API Reference](#8-http-api-reference)
9. [Authentication & Authorization](#9-authentication--authorization)
10. [VOD Pipeline](#10-vod-pipeline)
11. [Live Streaming Pipeline](#11-live-streaming-pipeline)
12. [Telemetry](#12-telemetry)
13. [Internal Packages](#13-internal-packages)
14. [Data Flow Diagrams](#14-data-flow-diagrams)
15. [Adding Features](#15-adding-features)
16. [Deployment](#16-deployment)
17. [Security Considerations](#17-security-considerations)
18. [Known Limitations](#18-known-limitations)
19. [Dependencies](#19-dependencies)

---

## 1. Architecture Overview

```
                    ┌──────────────────────────────────────────────────┐
                    │              Go HTTP Server (:8080)               │
                    │                                                    │
  Frontend ─────────┤  chi router                                       │
  (separate repo)   │  api.HandlerFromMux → 30 OpenAPI endpoints        │
                    │  /auth/google/*     → OAuth redirect flow         │
  OBS/Encoder ──────┤  /videos/*          → VOD HLS (JWT-protected)     │
     RTMP :1935     │  /live/*            → Live HLS (JWT, no-cache)    │
                    │  /thumbnails/*      → Thumbnail images (public)   │
                    │  /metrics           → Prometheus (requires login)  │
                    └──────────────────┬───────────────────────────────┘
                                       │
                    ┌──────────────────▼────────────────┐
                    │           PostgreSQL 15             │
                    │  users, videos, upload_chunks,      │
                    │  transcode_jobs,                    │
                    │  playback_sessions/events,          │
                    │  stream_keys, live_streams,         │
                    │  comments, chat_messages            │
                    └────────────────────────────────────┘

VOD path:  Upload → assemble → TranscodeWorker → FFmpeg → HLS fMP4 → data/hls/
Live path: OBS → RTMP → go-rtmp → FLV pipe → FFmpeg → HLS TS → data/live/
```

### Component layers

| Layer | Package | Purpose |
|---|---|---|
| `cmd/server` | Entry point | Wire config → DB → repos → services → chi router |
| `internal/api` | Generated code | Types, `ServerInterface`, `HandlerFromMux` (do not edit manually) |
| `internal/server` | API handlers | Implements `ServerInterface` — one file per domain |
| `internal/service` | Business logic | No HTTP, no SQL — pure domain operations |
| `internal/repository` | Data access | SQL queries, returns model structs |
| `internal/live` | Live ingest | RTMP server + per-stream FFmpeg session management |
| `internal/transcoder` | VOD encoding | FFmpeg/FFprobe wrappers |
| `internal/worker` | Job queue | Goroutine pool for transcode jobs |
| `internal/watchdog` | Process monitor | Detects stuck FFmpeg processes, resets stalled jobs |
| `internal/metrics` | Observability | Prometheus metric definitions + system collector |
| `internal/health` | Health probes | Liveness and readiness checks |
| `internal/middleware` | HTTP plumbing | JWT auth, rate limiting, request ID, metrics |

---

## 2. Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.22+ | Build & run |
| FFmpeg + FFprobe | 4.0+ | Video encoding (must be in `$PATH`) |
| Docker + Docker Compose | Any | Run PostgreSQL |

```bash
# macOS
brew install ffmpeg go
# Ubuntu/Debian
apt-get install ffmpeg golang
# Verify
ffmpeg -version && ffprobe -version && go version
```

---

## 3. Quick Start

```bash
# 1. Copy and fill in required env vars
cp .env.example .env

# 2. Start PostgreSQL
make db

# 3. Start the server (runs migrations automatically)
make serve
```

The server starts at `http://localhost:8080` (HTTP) and `rtmp://localhost:1935/live` (RTMP).

Migrations run automatically on startup — no manual SQL required.

### Makefile targets

| Target | Description |
|---|---|
| `make db` | Start PostgreSQL in Docker |
| `make stop` | Stop PostgreSQL |
| `make serve` | Run HTTP + RTMP server |
| `make dev` | Live-reload dev server (`air` or `go run`) |
| `make build` | Compile binaries to `bin/` |
| `make spec-validate` | Validate `definition/api.yaml` |
| `make spec-generate` | Regenerate `internal/api/api.gen.go` from spec |
| `make transcode INPUT=… OUTPUT=…` | CLI batch transcode (no server) |
| `make clean` | Remove `bin/` and `data/` |

---

## 4. Configuration

All config is read from environment variables. Copy `.env.example` to `.env` for local development — the Makefile includes `.env` automatically. Real environment variables always take precedence over `.env`.

The server **refuses to start** if `JWT_SECRET` is the dev default, `GOOGLE_CLIENT_ID` is empty, or `SESSION_COOKIE_SECRET` is shorter than 32 characters.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://philos:philos@localhost:5433/philos_video?sslmode=disable` | PostgreSQL DSN |
| `DATA_DIR` | `./data` | Root for all video storage |
| `WORKER_COUNT` | `2` | Concurrent transcode workers |
| `JWT_SECRET` | dev default | **Must be changed in production.** Min 32 chars. |
| `JWT_EXPIRY` | `1h` | Playback token lifetime (Go duration: `30m`, `2h`, etc.) |
| `RTMP_PORT` | `1935` | RTMP ingest port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `text` | `text` or `json` |
| `GOOGLE_CLIENT_ID` | — | Required. Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | — | Required. Google OAuth 2.0 client secret |
| `OAUTH_REDIRECT_URL` | — | Required. Full callback URL (e.g. `https://api.example.com/auth/google/callback`) |
| `SESSION_COOKIE_SECRET` | — | Required. Min 32 chars, signs the browser session cookie |
| `SESSION_COOKIE_SECURE` | `false` | Set `true` in production (HTTPS only cookie) |
| `DEFAULT_UPLOAD_QUOTA_BYTES` | `10737418240` | Per-user upload quota (10 GiB). `0` = unlimited. |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins (e.g. `https://app.example.com`) |
| `GOLIVE_WHITELIST` | — | Comma-separated emails allowed to manage stream keys |

**Production example:**

```bash
export PORT=8080
export DATABASE_URL="postgres://user:pass@db-host:5432/philos_video?sslmode=require"
export DATA_DIR="/mnt/video-storage"
export WORKER_COUNT=4
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_EXPIRY="2h"
export SESSION_COOKIE_SECRET="$(openssl rand -hex 32)"
export SESSION_COOKIE_SECURE=true
export CORS_ORIGINS="https://app.example.com"
export GOOGLE_CLIENT_ID="…"
export GOOGLE_CLIENT_SECRET="…"
export OAUTH_REDIRECT_URL="https://api.example.com/auth/google/callback"
./bin/server
```

---

## 5. Directory Structure

```
philos-video/
├── cmd/
│   ├── server/main.go          # Entry point: wires all components, registers routes
│   ├── migrate/main.go         # Standalone migration runner (goose)
│   └── transcode/main.go       # Standalone CLI for batch transcoding
│
├── definition/
│   ├── api.yaml                # OpenAPI 3.0.3 spec (single file, source of truth)
│   ├── oapi-codegen.yaml       # Code generation config
│   └── src/                   # Split YAML files (documentation / future bundling)
│
├── internal/
│   ├── api/api.gen.go          # Generated: types, ServerInterface, HandlerFromMux (do not edit)
│   │
│   ├── config/config.go        # Env var parsing → Config struct
│   ├── database/postgres.go    # sql.Open + Migrate (inlined SQL)
│   │
│   ├── models/models.go        # All DB-facing structs + status constants
│   │
│   ├── repository/             # One file per DB table
│   │   ├── user_repo.go
│   │   ├── video_repo.go
│   │   ├── upload_repo.go
│   │   ├── job_repo.go
│   │   ├── session_repo.go
│   │   ├── event_repo.go
│   │   ├── stream_key_repo.go
│   │   ├── live_stream_repo.go
│   │   ├── comment_repo.go
│   │   └── chat_message_repo.go
│   │
│   ├── service/                # Business logic (no HTTP, no SQL)
│   │   ├── video_service.go
│   │   ├── upload_service.go
│   │   ├── transcode_service.go
│   │   ├── session_service.go  # JWT generation + PlaybackClaims struct
│   │   ├── comment_service.go
│   │   ├── chat_hub.go         # In-memory fan-out for live chat SSE
│   │   ├── oauth_service.go    # Google OAuth exchange + user info fetch
│   │   └── user_session_service.go  # Browser session cookie (JWT)
│   │
│   ├── server/                 # Implements api.ServerInterface (one file per domain)
│   │   ├── server.go           # Server struct + Params + New()
│   │   ├── errors.go           # writeJSON, writeError, decodeJSON helpers
│   │   ├── health.go           # GetHealth, GetHealthReady
│   │   ├── auth.go             # GetMe, Logout, GoogleLoginHandler, GoogleCallbackHandler
│   │   ├── video.go            # ListVideos, GetVideo, GetVideoStatus, DeleteVideo, UpdateVideo
│   │   ├── upload.go           # InitUpload, ReceiveChunk, GetUploadStatus, UploadThumbnail
│   │   ├── session.go          # CreateVideoSession, CreateLiveSession
│   │   ├── comment.go          # ListComments, AddComment, DeleteComment
│   │   ├── live.go             # ListLiveStreams, GetLiveStream, GetLiveViewers, EndLiveStream
│   │   ├── chat.go             # SendChatMessage, ChatStream (SSE), ListChatMessages
│   │   ├── stream_key.go       # ListStreamKeys, CreateStreamKey, DeactivateStreamKey, UpdateStreamKey
│   │   └── telemetry.go        # PostTelemetryEvents
│   │
│   ├── middleware/             # HTTP middleware
│   │   ├── auth.go             # JWT validation for HLS file serving (VOD + live)
│   │   ├── auth_user.go        # User session middleware (OptionalUser, RequireUser, RequireUserAPI)
│   │   ├── golive_gate.go      # GOLIVE_WHITELIST check (used inside stream key handlers)
│   │   ├── metrics_mw.go       # Prometheus HTTP metrics middleware
│   │   ├── ratelimit.go        # Per-IP fixed-window rate limiter
│   │   └── request_id.go       # X-Request-ID injection
│   │
│   ├── live/                   # RTMP ingest + live transcoding
│   │   ├── rtmp_server.go      # go-rtmp server wrapper
│   │   ├── rtmp_handler.go     # Per-connection RTMP handler
│   │   ├── manager.go          # Stream lifecycle + in-memory session map
│   │   └── transcode_session.go# FFmpeg process + FLV framing
│   │
│   ├── transcoder/             # FFmpeg/FFprobe wrappers for VOD
│   │   ├── probe.go
│   │   ├── ladder.go
│   │   ├── encode.go
│   │   ├── segment.go
│   │   └── manifest.go
│   │
│   ├── worker/transcode_worker.go  # Goroutine pool reading job channel
│   ├── watchdog/watchdog.go    # Detects stuck FFmpeg processes + stalled jobs
│   ├── health/health.go        # Liveness + readiness checks
│   ├── metrics/metrics.go      # Prometheus metric definitions + system collector
│   └── logging/logging.go      # slog setup (level + format from config)
│
├── data/                       # Runtime-generated, gitignored
│   ├── chunks/{upload_id}/     # Raw uploaded chunks (deleted after assembly)
│   ├── raw/{upload_id}/        # Assembled input file (deleted after transcode)
│   ├── hls/{video_id}/         # Final VOD output served at /videos/{id}/…
│   ├── live/{stream_id}/       # Live HLS output served at /live/{id}/…
│   └── thumbnails/             # Uploaded thumbnail images
│
├── go.mod / go.sum
├── Makefile
├── CLAUDE.md
└── docker-compose.yml          # postgres:15 on port 5433
```

### Data directory layout (at runtime)

```
data/
├── chunks/{upload_id}/
│   ├── 00000           ← raw chunk bytes
│   ├── 00001
│   └── ...
├── raw/{upload_id}/
│   └── original.mp4    ← assembled file (deleted after transcode)
├── hls/{video_id}/
│   ├── master.m3u8     ← served at /videos/{video_id}/master.m3u8
│   ├── 720p/
│   │   ├── playlist.m3u8
│   │   ├── init.mp4    ← fMP4 init segment
│   │   └── segment_0000.m4s ...
│   ├── 480p/ ...
│   └── 360p/ ...
├── live/{stream_id}/
│   ├── master.m3u8     ← pre-written at stream start
│   ├── 720p/
│   │   ├── playlist.m3u8  ← sliding window (5 segments)
│   │   └── segment_0000.ts ...
│   ├── 480p/ ...
│   └── 360p/ ...
└── thumbnails/
    └── {upload_id}.jpg
```

---

## 6. OpenAPI Workflow

The API contract lives in `definition/api.yaml` (OpenAPI 3.0.3). Code generation via `oapi-codegen` produces `internal/api/api.gen.go` which contains all model types, the `ServerInterface` (30 methods), and `HandlerFromMux` for automatic chi route registration.

**Never edit `internal/api/api.gen.go` by hand** — it is always regenerated from the spec.

### Adding a new endpoint

1. **Edit `definition/api.yaml`** — add the path, method, request/response schemas
2. **Regenerate:** `make spec-generate`
3. **Implement:** add the new method to the appropriate file in `internal/server/`

```bash
# After editing definition/api.yaml:
make spec-validate  # catch spec errors early
make spec-generate  # regenerates internal/api/api.gen.go
# The compiler will now report missing interface methods — implement them
```

### Code generation config (`definition/oapi-codegen.yaml`)

```yaml
package: api
generate:
  - types
  - chi-server
output: ../internal/api/api.gen.go
```

### Generated artifacts

- **Model types** — Go structs for all request/response schemas (e.g. `Video`, `User`, `ChatMessage`)
- **`ServerInterface`** — 30-method interface that `*server.Server` must implement
- **`HandlerFromMux(si ServerInterface, r chi.Router)`** — registers all routes on a chi router
- **`ServerInterfaceWrapper`** — adapts the interface to chi's handler signature, extracts path/query params

### Path parameter extraction

oapi-codegen extracts path parameters from the chi URL context and passes them as typed function arguments. **Do not** use `chi.URLParam(r, "id")` in handlers — they arrive as direct arguments:

```go
// Generated interface method:
GetVideo(w http.ResponseWriter, r *http.Request, id string)

// Implementation — id is already extracted:
func (s *Server) GetVideo(w http.ResponseWriter, r *http.Request, id string) {
    video, err := s.videoSvc.GetByID(r.Context(), id)
    ...
}
```

### Query parameter structs

Query parameters arrive as typed structs defined in the spec:

```go
// Generated:
type ListVideosParams struct {
    Limit  *int    `form:"limit"`
    Offset *int    `form:"offset"`
    Status *string `form:"status"`
}

// Implementation:
func (s *Server) ListVideos(w http.ResponseWriter, r *http.Request, params api.ListVideosParams) {
    limit := 20
    if params.Limit != nil { limit = *params.Limit }
    ...
}
```

---

## 7. Database Schema

Schema is managed by [goose](https://github.com/pressly/goose) migrations in `internal/database/migrations/`. They are embedded into the binary at build time and applied automatically on startup.

### Managing migrations

| Command | Description |
|---|---|
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the last migration |
| `make migrate-status` | Show applied / pending migrations |
| `make migrate-new name=<name>` | Scaffold a new numbered SQL migration file |

### `users`

```sql
id                   TEXT PRIMARY KEY          -- usr_{hex}
google_sub           TEXT UNIQUE NOT NULL      -- Google subject identifier
email                TEXT NOT NULL
name                 TEXT NOT NULL
picture              TEXT                      -- Google profile picture URL
upload_quota_bytes   BIGINT NOT NULL DEFAULT 10737418240  -- 10 GiB
used_bytes           BIGINT NOT NULL DEFAULT 0
created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `videos`

```sql
id           TEXT PRIMARY KEY              -- same as upload_id (hex)
user_id      TEXT REFERENCES users(id)
title        TEXT NOT NULL
visibility   TEXT NOT NULL DEFAULT 'public'  -- public | private
status       TEXT NOT NULL DEFAULT 'uploading'
             -- uploading → processing → ready | failed
width        INT
height       INT
duration     TEXT                          -- e.g. "00:03:47.00"
codec        TEXT
hls_path     TEXT                          -- relative from DATA_DIR
thumbnail    TEXT                          -- relative path in thumbnails/
view_count   BIGINT NOT NULL DEFAULT 0
created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `upload_chunks`

```sql
upload_id    TEXT                          -- matches videos.id
chunk_number INT
received     BOOLEAN NOT NULL DEFAULT FALSE
PRIMARY KEY (upload_id, chunk_number)
```

### `transcode_jobs`

```sql
id         TEXT PRIMARY KEY
video_id   TEXT NOT NULL REFERENCES videos(id)
status     TEXT NOT NULL DEFAULT 'queued'  -- queued → running → completed | failed
stage      TEXT                            -- current FFmpeg stage name
progress   DOUBLE PRECISION DEFAULT 0      -- 0.0 – 1.0
error      TEXT
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

Progress milestones: `probe=0.05`, `prepare=0.10`, `encode per profile 0.10–0.80`, `segment=0.85`, `packaging=0.95`, `done=1.0`

### `playback_sessions`

```sql
id             TEXT PRIMARY KEY           -- sess_{hex}
video_id       TEXT REFERENCES videos(id) -- null for live
stream_id      TEXT                       -- set for live sessions
token          TEXT NOT NULL
device_type    TEXT
user_agent     TEXT
ip_address     TEXT
started_at     TIMESTAMPTZ DEFAULT NOW()
last_active_at TIMESTAMPTZ DEFAULT NOW()  -- debounced: updated at most every 30s
ended_at       TIMESTAMPTZ
status         TEXT DEFAULT 'active'      -- active | ended
```

### `playback_events`

```sql
id                   BIGSERIAL PRIMARY KEY
session_id           TEXT NOT NULL REFERENCES playback_sessions(id)
video_id             TEXT NOT NULL
event_type           TEXT NOT NULL
  -- playback_start, segment_downloaded, quality_change,
  -- rebuffer_start, rebuffer_end, heartbeat, playback_end, playback_error
timestamp            TIMESTAMPTZ DEFAULT NOW()
segment_number       INTEGER
segment_quality      TEXT          -- 720p | 480p | 360p
segment_bytes        BIGINT
download_time_ms     INTEGER
throughput_bps       BIGINT
current_quality      TEXT
buffer_length        DOUBLE PRECISION
playback_position    DOUBLE PRECISION
rebuffer_duration_ms INTEGER
quality_from         TEXT
quality_to           TEXT
error_code           TEXT
error_message        TEXT

-- Indexes: session_id, (event_type, timestamp), (video_id, timestamp)
```

### `stream_keys`

```sql
id           TEXT PRIMARY KEY    -- sk_{hex}
user_id      TEXT REFERENCES users(id)
user_label   TEXT NOT NULL
is_active    BOOLEAN DEFAULT TRUE
record_vod   BOOLEAN DEFAULT TRUE
created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `live_streams`

```sql
id            TEXT PRIMARY KEY       -- ls_{hex}
stream_key_id TEXT NOT NULL REFERENCES stream_keys(id)
user_id       TEXT REFERENCES users(id)
title         TEXT NOT NULL
status        TEXT DEFAULT 'waiting'  -- waiting → live → ended
source_width  INT
source_height INT
source_codec  TEXT
source_fps    TEXT
hls_path      TEXT
video_id      TEXT                    -- set when VOD recording is created
started_at    TIMESTAMPTZ
ended_at      TIMESTAMPTZ
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- Indexes: status, stream_key_id
```

### `comments`

```sql
id         TEXT PRIMARY KEY    -- cmt_{hex}
video_id   TEXT NOT NULL REFERENCES videos(id)
user_id    TEXT REFERENCES users(id)
user_name  TEXT NOT NULL
picture    TEXT
body       TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `chat_messages`

```sql
id          TEXT PRIMARY KEY    -- chat_{hex}
stream_id   TEXT NOT NULL
user_id     TEXT REFERENCES users(id)
user_name   TEXT NOT NULL
picture     TEXT
body        TEXT NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- Index: (stream_id, created_at)
```

---

## 8. HTTP API Reference

All API endpoints are defined in `definition/api.yaml` and accessible via the generated chi routes. JSON request/response unless noted.

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/auth/google/login` | — | Redirect to Google OAuth consent page |
| `GET` | `/auth/google/callback` | — | OAuth callback; sets session cookie; redirects |
| `POST` | `/auth/logout` | optional | Clear session cookie |
| `GET` | `/api/v1/me` | required | Current user info |

`GET /api/v1/me` response:
```json
{
  "id": "usr_abc123",
  "email": "user@example.com",
  "name": "Alice",
  "picture": "https://…",
  "used_bytes": 1073741824,
  "upload_quota_bytes": 10737418240
}
```

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness: always `200 OK` if process is running |
| `GET` | `/health/ready` | Readiness: checks DB, FFmpeg, disk, RTMP port |

### Upload

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/uploads` | required | Initialise chunked upload |
| `PUT` | `/api/v1/uploads/{upload_id}/chunks/{chunk_number}` | required | Send one raw chunk |
| `GET` | `/api/v1/uploads/{upload_id}/status` | required | `{"received":3,"total":5}` |
| `POST` | `/api/v1/uploads/{upload_id}/thumbnail` | required | Upload thumbnail (multipart/form-data, field: `thumbnail`) |

`POST /api/v1/uploads` request:
```json
{
  "filename": "video.mp4",
  "title": "My Video",
  "visibility": "public",
  "total_chunks": 12,
  "file_size": 62914560
}
```
Response: `201 Created`
```json
{ "upload_id": "a1b2c3d4", "total_chunks": 12 }
```

### Videos

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/videos` | optional | List videos. Query: `?limit=20&offset=0&status=ready` |
| `GET` | `/api/v1/videos/{id}` | optional | Single video record |
| `GET` | `/api/v1/videos/{id}/status` | optional | Video + job + progress (0.0–1.0) |
| `DELETE` | `/api/v1/videos/{id}` | required | Delete video + all HLS files |
| `PATCH` | `/api/v1/videos/{id}` | required | Update `title` or `visibility` |

`GET /api/v1/videos/{id}/status` response:
```json
{
  "video":    { "id": "…", "title": "…", "status": "processing", … },
  "job":      { "id": "…", "status": "running", "stage": "encode:720p", "progress": 0.35 },
  "progress": 0.35
}
```

### Playback Sessions

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/videos/{id}/sessions` | optional | Create JWT session for VOD playback |
| `POST` | `/api/v1/live/{stream_id}/sessions` | optional | Create JWT session for live playback |

Request body (both):
```json
{ "device_type": "desktop", "user_agent": "optional override" }
```
Response `201 Created`:
```json
{
  "session_id":       "sess_abc123",
  "manifest_url":     "/videos/{id}/master.m3u8?token=eyJ…",
  "token":            "eyJ…",
  "token_expires_at": "2025-03-10T13:00:00Z",
  "telemetry_url":    "/api/v1/sessions/sess_abc123/events"
}
```

### Telemetry

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/sessions/{session_id}/events` | — | Batch-insert playback events (session-validated) |

Request body (up to 1000 events per batch):
```json
{
  "events": [
    { "event_type": "heartbeat", "current_quality": "720p", "buffer_length": 8.5 },
    { "event_type": "quality_change", "quality_from": "720p", "quality_to": "480p" }
  ]
}
```

Event types: `playback_start`, `segment_downloaded`, `quality_change`, `rebuffer_start`, `rebuffer_end`, `heartbeat`, `playback_end`, `playback_error`

### Comments

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/videos/{video_id}/comments` | optional | List comments. Query: `?limit=20&offset=0` |
| `POST` | `/api/v1/videos/{video_id}/comments` | required | Add comment `{"body":"…"}` |
| `DELETE` | `/api/v1/videos/{video_id}/comments/{comment_id}` | required | Delete own comment |

### Live Streams

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/live` | — | List all live streams |
| `GET` | `/api/v1/live/{stream_id}` | — | Single live stream record |
| `GET` | `/api/v1/live/{stream_id}/viewers` | — | `{"count": 42}` |
| `POST` | `/api/v1/live/{stream_id}/end` | required (owner) | Manually end stream + trigger VOD |

### Live Chat

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/live/{stream_id}/chat` | required | Send message `{"body":"…"}` |
| `GET` | `/api/v1/live/{stream_id}/chat/stream` | — | SSE stream (`text/event-stream`): history on connect, then new messages |
| `GET` | `/api/v1/live/{stream_id}/chat` | — | List recent messages. Query: `?limit=50` |

The SSE stream sends an initial `data:` event with `{"history":[…]}`, then individual `ChatMessage` objects as they arrive. A `heartbeat` comment is sent every 30s.

### Stream Keys

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/stream-keys` | required + whitelist | Create key `{"label":"OBS","record_vod":true}` |
| `GET` | `/api/v1/stream-keys` | required + whitelist | List active keys |
| `DELETE` | `/api/v1/stream-keys/{id}` | required + whitelist | Deactivate key |
| `PATCH` | `/api/v1/stream-keys/{id}` | required + whitelist | Update `record_vod` |

Stream key endpoints require the authenticated user's email to be in `GOLIVE_WHITELIST`.

### File Serving (Not in OpenAPI spec — manually registered)

| Pattern | Auth check | Serves from |
|---|---|---|
| `GET /thumbnails/*` | None | `{DATA_DIR}/thumbnails/` |
| `GET /videos/*` | JWT `vid` claim == path segment | `{DATA_DIR}/hls/` |
| `GET /live/*` | JWT `stid` claim == path segment | `{DATA_DIR}/live/` |

Token must be passed as `?token=<jwt>` on every HLS request. Live routes add `Cache-Control: no-cache`.

---

## 9. Authentication & Authorization

### Google OAuth flow

```
Client → GET /auth/google/login
      ← 302 redirect to Google consent page

Google → GET /auth/google/callback?code=…&state=…
Server:  Exchange code for tokens
         Fetch user profile (email, name, picture)
         Upsert user record (users table)
         Sign browser session cookie (HS256 JWT)
      ← 302 redirect to / (or ?return= param)
```

The session cookie contains a short JWT with `{user_id}`. `UserAuthMiddleware.OptionalUser` validates the cookie on every request and populates `ctx` — all handlers call `middleware.CurrentUser(ctx)` to retrieve the user.

### Playback JWT structure

```json
{
  "jti":  "sess_abc123",
  "iat":  1710000000,
  "exp":  1710003600,
  "sid":  "sess_abc123",   -- session ID
  "vid":  "a1b2c3…",       -- video ID (VOD sessions only)
  "stid": "ls_d4e5f6…"     -- stream ID (live sessions only)
}
```

The HLS auth middleware (`internal/middleware/auth.go`) validates the JWT on every file request and asserts that the claimed `vid`/`stid` matches the path segment — a VOD token cannot be used for a different video or any live stream.

### User auth middleware levels

| Method | Behaviour |
|---|---|
| `OptionalUser` | Populates user context if cookie is valid; passes through even if not signed in |
| `RequireUser` | Redirects to `/auth/google/login` if not signed in (for UI routes) |
| `RequireUserAPI` | Returns `401 Unauthorized` JSON if not signed in (for API routes) |

In the current setup, `OptionalUser` is applied globally via chi middleware. Handlers that require auth call `middleware.CurrentUser(ctx)` and return `401` if nil.

---

## 10. VOD Pipeline

### Upload phase

1. Client calls `POST /api/v1/uploads` → server creates `videos` and `upload_chunks` records, returns `upload_id`
2. Client splits file into chunks (any size up to 256 MiB each), sends via `PUT …/chunks/{n}` (raw body, `application/octet-stream`)
3. Each chunk is written to `data/chunks/{upload_id}/{n}`
4. When the last chunk lands, `UploadService.assemble()` runs in a goroutine:
   - Concatenates all chunks in order → `data/raw/{upload_id}/original.{ext}`
   - Deletes chunk files
   - Creates `TranscodeJob` record (status=queued)
   - Sends `job_id` to the worker channel

### Transcode phase

The `TranscodeWorker` goroutine pool reads from the job channel:

1. **Probe** (`internal/transcoder/probe.go`): runs `ffprobe -print_format json -show_streams -show_format` → extracts width, height, codec, duration
2. **Build Ladder** (`internal/transcoder/ladder.go`): filters the 3-profile ladder to only include resolutions ≤ source
3. **Per profile** (`internal/transcoder/encode.go` + `segment.go`):
   - Encode: `ffmpeg … -c:v libx264 -preset medium` → `data/hls/{id}/{profile}/intermediate.mp4`
   - Segment: `ffmpeg -f hls -hls_segment_type fmp4 -hls_time 4` → playlist + `.m4s` segments
   - Delete `intermediate.mp4`
4. **Master Manifest** (`internal/transcoder/manifest.go`): writes `data/hls/{id}/master.m3u8`
5. Updates `videos.status = ready`, `videos.hls_path`, job status = completed

### Encoding ladder

| Profile | Resolution | Video | Audio | MaxRate |
|---------|-----------|-------|-------|---------|
| 720p | 1280×720 | 2500 kbps | 128 kbps | 2500 kbps |
| 480p | 854×480 | 1000 kbps | 96 kbps | 1000 kbps |
| 360p | 640×360 | 400 kbps | 64 kbps | 400 kbps |

Profiles whose height exceeds the source height are skipped.

### Serving VOD

`http.FileServer` serves `data/hls/` with:
- MIME types set for `.m3u8` → `application/vnd.apple.mpegurl`, `.m4s` → `video/iso.bmff`
- JWT validation via `RequirePlaybackToken` middleware

---

## 11. Live Streaming Pipeline

### Broadcaster setup

1. `POST /api/v1/stream-keys` with `{"label":"OBS"}` → copy the `sk_…` value
2. In OBS: **Settings → Stream → Service: Custom**
   - Server: `rtmp://localhost:1935/live`
   - Stream Key: `sk_…`
3. Start Streaming

### RTMP ingest path

```
OBS ──RTMP──▶ RTMPServer (internal/live/rtmp_server.go)
                 └─ go-rtmp → per connection: rtmpHandler
                      OnPublish() → Manager.StartStream(streamKey)
                        1. Validate stream key in DB
                        2. Create live_streams record (status=live)
                        3. newTranscodeSession() → spawn FFmpeg
                        4. Write FLV file header to FFmpeg stdin
                      OnVideo(ts, payload) → writeTag(0x09, …)
                      OnAudio(ts, payload) → writeTag(0x08, …)
                      OnClose() → Manager.EndStream()
```

### FLV framing

RTMP message payloads are raw FLV tag data bytes. `transcodeSession.writeTag()` prepends the 11-byte FLV tag header (type + data size + timestamp + stream ID) and appends the 4-byte `PreviousTagSize`.

### Live FFmpeg pipeline

```
FFmpeg stdin (FLV pipe)
  ↓
-f flv -i pipe:0
  ↓
-filter_complex "[0:v]split=3[v720][v480][v360]; scale to 1280:720, 854:480, 640:360"
  ↓
-f hls
  -hls_time 2                  (2-second segments — lower latency)
  -hls_list_size 5             (sliding window: 5 segments)
  -hls_flags delete_segments+independent_segments+append_list
  -hls_segment_type mpegts     (MPEG-TS for live, not fMP4)
  ↓
data/live/{stream_id}/720p/playlist.m3u8 + *.ts
data/live/{stream_id}/480p/playlist.m3u8
data/live/{stream_id}/360p/playlist.m3u8
data/live/{stream_id}/master.m3u8  ← written upfront
```

All live profiles use `libx264` with `-preset veryfast -tune zerolatency` (GOP=60) for ~2–4s end-to-end latency.

### Stream end & VOD conversion

When OBS disconnects or `POST /api/v1/live/{id}/end` is called:

1. `Manager.EndStream()`: close FFmpeg stdin → FFmpeg flushes → appends `#EXT-X-ENDLIST` → updates `live_streams.status = ended`
2. `Manager.convertToVOD()` (goroutine): creates a `videos` record pointing at `live/{stream_id}`, immediately playable via `/api/v1/videos/{video_id}`

`EndAllStreams()` is called on SIGTERM so all live streams terminate cleanly.

---

## 12. Telemetry

The telemetry handler (`internal/server/telemetry.go`) validates the session, batch-inserts events to `playback_events`, and increments Prometheus counters.

### Event batch insert

```
POST /api/v1/sessions/{session_id}/events
  → validate session exists + status=active
  → backfill missing timestamps with server time
  → eventRepo.BatchInsert() → single INSERT with UNNEST arrays
  → update Prometheus counters per event_type
```

### Prometheus counters updated per event

| Event | Metric |
|---|---|
| any | `telemetry_events_received_total{event_type}` |
| `playback_start` | `playback_ttff_seconds` (histogram) |
| `rebuffer_start` | `playback_rebuffer_total` (counter) |
| `rebuffer_end` | `playback_rebuffer_duration_seconds` (histogram) |
| `quality_change` | `playback_quality_switches_total{direction}` (up/down) |
| `playback_error` | `playback_errors_total{error_code}` |

---

## 13. Internal Packages

### `internal/server`

Implements `api.ServerInterface`. Each file handles one domain:
- All handlers use `writeJSON(w, v, status)` / `writeError(w, msg, status)` from `errors.go`
- Handlers check auth with `middleware.CurrentUser(ctx)` — return `401` if nil when auth is required
- The `decodeJSON(r, dst)` helper limits body reads and returns a clear error on malformed JSON

### `internal/repository`

One struct per table, SQL uses `$N` positional parameters (PostgreSQL). Most methods accept `context.Context`. Nullable column helpers (`ns`, `ni`, `ni64`, `nf64`) are defined in `event_repo.go` and shared across the package.

### `internal/service`

Pure business logic — no HTTP types, no `*sql.DB`. Services receive repos via constructor injection.

- `ChatHub`: in-memory fan-out for live chat. Manages per-stream subscriber channels, persists messages via `ChatMessageRepo`.
- `SessionService`: issues and validates playback JWTs, creates/updates `playback_sessions`.
- `UserSessionService`: issues and validates the browser session cookie JWT (separate from playback JWTs).
- `OAuthService`: wraps `golang.org/x/oauth2` for the Google flow.

### `internal/live`

`Manager` is the sole owner of all active `transcodeSession` objects. `RTMPHandler` (one per TCP connection) holds a `Manager` reference — no direct DB access from the handler.

### `internal/watchdog`

Runs on a 30-second tick. Detects FFmpeg processes that have stopped writing segments (stalled live streams) and resets `transcode_jobs` stuck in `running` status back to `queued` so the worker can retry.

---

## 14. Data Flow Diagrams

### VOD Upload → Playback

```
Client              UploadService         DB              Worker
  │                      │                 │                 │
  │─ POST /uploads ──────▶│ create records  │                 │
  │◀─ {upload_id} ────────│                 │                 │
  │─ PUT …/chunks/0 ──────▶│ save to disk    │                 │
  │─ PUT …/chunks/N ──────▶│ last chunk!     │                 │
  │                       │─ assemble ──────▶│ create job      │
  │                       │                 │──── jobCh ─────▶│
  │                       │                 │                 │ Process()
  │                       │                 │◀── update prog──│
  │─ GET /videos/{id}/status ───────────────▶│                 │
  │◀─ {progress:0.35} ──────────────────────│                 │
  │                       │                 │◀── status=ready─│
  │─ POST /videos/{id}/sessions ────────────▶│ create session  │
  │◀─ {manifest_url,token} ─────────────────│                 │
  │─ GET /videos/{id}/master.m3u8?token=… ───── auth check ──▶│
  │◀─ HLS manifest ──────────────────────────────────────────  │
```

### Live Broadcast → Viewer

```
OBS              RTMPHandler          Manager              FFmpeg
 │                    │                  │                    │
 │─ RTMP publish ────▶│ OnPublish()       │                    │
 │  key=sk_…          │─ StartStream() ──▶│ spawn FFmpeg ──────▶│
 │                    │                  │─ write FLV header ──▶│
 │─ video packet ────▶│ OnVideo()         │                    │
 │                    │─ WriteVideo() ────▶│─ writeTag(0x09) ──▶│
 │─ audio packet ────▶│ OnAudio()         │                    │
 │                    │─ WriteAudio() ────▶│─ writeTag(0x08) ──▶│
 │                    │                  │               produces HLS
 │─ RTMP disconnect ─▶│ OnClose()         │                    │
 │                    │─ EndStream() ─────▶│ close stdin ───────▶│
 │                    │                  │   FFmpeg exits       │
 │                    │                  │ #EXT-X-ENDLIST       │
 │                    │                  │ create VOD record    │

Viewer               AuthMiddleware       server.CreateLiveSession
 │                       │                      │
 │─ POST /live/{id}/sessions ────────────────────▶│ create session + JWT
 │◀─ {manifest_url,token} ───────────────────────│
 │─ GET /live/{id}/master.m3u8?token=… ──── verify stid claim
 │◀─ HLS manifest (no-cache) ─────────────────────
```

---

## 15. Adding Features

### Adding a new API endpoint (OpenAPI workflow)

1. Edit `definition/api.yaml` — add the path + schemas
2. `make spec-validate` — catch YAML/spec errors
3. `make spec-generate` — regenerate `internal/api/api.gen.go`
4. Add SQL if needed: append to the relevant migration in `internal/database/migrations/`
5. Add model fields if needed: `internal/models/models.go`
6. Add repository method: `internal/repository/{domain}_repo.go`
7. Add service method: `internal/service/{domain}_service.go`
8. Implement the new `ServerInterface` method: `internal/server/{domain}.go`

The compiler will report missing interface methods after step 3 — follow the errors.

### Adding a new video quality profile

Edit `internal/transcoder/ladder.go`. Add a `Profile` to `defaultLadder`. Also update:
- The live FFmpeg args in `internal/live/transcode_session.go`
- The `liveMasterPlaylist` constant in the same file

### Adding a new telemetry event type

1. Add columns to `playback_events` in a new migration file
2. Add the field to `models.PlaybackEvent`
3. Update `event_repo.go`'s `BatchInsert` (increment `numCols`, add the new column)
4. Handle in `internal/server/telemetry.go`'s Prometheus switch statement

### Changing JWT expiry

`JWT_EXPIRY` is a Go duration string (`30m`, `2h`, `12h`). Tokens are embedded in HLS manifest URLs — if a token expires mid-playback, segment requests will fail. For long videos, use a generous expiry. Token refresh is not implemented.

---

## 16. Deployment

### Build production binaries

```bash
make build
# Produces: bin/server
```

### Docker Compose (PostgreSQL only)

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:15
    ports: ["5433:5432"]
    environment:
      POSTGRES_DB: philos_video
      POSTGRES_USER: philos
      POSTGRES_PASSWORD: philos
    volumes: [pgdata:/var/lib/postgresql/data]
```

Port 5433 (not 5432) avoids conflicts with a local Postgres install.

### Systemd service

```ini
[Unit]
Description=philos-video API server
After=network.target postgresql.service

[Service]
User=www-data
WorkingDirectory=/opt/philos-video
ExecStart=/opt/philos-video/bin/server
EnvironmentFile=/opt/philos-video/.env
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Nginx reverse proxy

```nginx
server {
    listen 443 ssl;
    server_name api.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        # Required for live chat SSE
        proxy_buffering off;
        proxy_cache off;
        # Required for large chunk uploads
        client_max_body_size 256m;
    }
}

# RTMP (TCP stream proxy)
stream {
    server {
        listen 1935;
        proxy_pass 127.0.0.1:1935;
    }
}
```

Set `CORS_ORIGINS=https://app.example.com` when the frontend is on a different origin.

---

## 17. Security Considerations

| Area | Status | Notes |
|---|---|---|
| `JWT_SECRET` | Dev default blocked at startup | Set a 32+ char random secret in production |
| Playback tokens | Bound to specific video/stream ID | Prevents token reuse across resources |
| Session cookie | HttpOnly, signed HS256 | Set `SESSION_COOKIE_SECURE=true` in production (HTTPS) |
| CORS | Configurable via `CORS_ORIGINS` | Defaults to `*` — restrict to frontend origin in production |
| Stream keys | `GOLIVE_WHITELIST` controls access | Only whitelisted emails can create/manage stream keys |
| Rate limiting | Per-IP, in-process | Applied on upload init, session create, comment/chat post |
| SQL injection | Parameterized queries (`$N`) | Protected throughout |
| FFmpeg injection | User data not in FFmpeg args | Filenames/titles are stored in DB, not passed to shell |
| Upload quota | Per-user byte limit | `DEFAULT_UPLOAD_QUOTA_BYTES`, enforced before chunk assembly |
| Body size limits | `http.MaxBytesReader` | Set per handler (chunks: 256 MiB, JSON bodies: 64 KiB) |

---

## 18. Known Limitations

1. **Safari native HLS**: JWT `?token=` query params work for the master playlist but may not be forwarded for sub-playlists fetched natively by Safari. HLS.js handles this correctly via `xhrSetup`.

2. **Live stream audio assumption**: The live FFmpeg pipeline assumes the RTMP stream contains at least one audio track. Video-only streams will cause FFmpeg to fail.

3. **No token refresh**: Tokens expire per `JWT_EXPIRY`. For long videos or live streams, use a generous expiry (e.g., `12h`).

4. **In-memory live sessions**: Active transcode sessions are lost on server restart. `live_streams` records will be stuck in `status=live` — run `UPDATE live_streams SET status='ended' WHERE status='live'` after an unclean shutdown.

5. **No CDN integration**: HLS files are served directly from the Go process. For production traffic, put a CDN (CloudFront, Fastly) in front of `/videos/` and `/live/`, or serve from object storage (S3, GCS).

6. **Local filesystem only**: Transcode workers and live FFmpeg write to `DATA_DIR` on local disk. Network storage (NFS, etc.) may have issues with live segment creation timing.

---

## 19. Dependencies

### Go modules (direct)

| Module | Version | Purpose |
|---|---|---|
| `github.com/go-chi/chi/v5` | v5.2.5 | HTTP router |
| `github.com/go-chi/cors` | v1.2.2 | CORS middleware |
| `github.com/oapi-codegen/runtime` | v1.4.0 | oapi-codegen runtime (param extraction) |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT signing and validation |
| `github.com/lib/pq` | v1.11.2 | PostgreSQL driver |
| `github.com/caarlos0/env/v11` | v11.4.0 | Env var config parsing |
| `github.com/google/uuid` | v1.6.0 | UUID generation |
| `github.com/pressly/goose/v3` | v3.27.0 | Database migrations |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics |
| `github.com/yutopp/go-rtmp` | v0.0.7 | RTMP server |
| `golang.org/x/oauth2` | v0.36.0 | Google OAuth 2.0 |

### System tools (must be in `$PATH`)

| Tool | Version | Purpose |
|---|---|---|
| `ffmpeg` | 4.0+ | Video encoding, segmentation, live transcoding |
| `ffprobe` | 4.0+ | Video metadata extraction |
