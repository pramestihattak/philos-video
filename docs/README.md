# PramTube — Developer Documentation

> **Overview & quick start:** [`../README.md`](../README.md)

A self-hosted video streaming platform written in Go. Supports chunked upload, server-side transcoding to adaptive-bitrate HLS (VOD), live RTMP ingest with real-time HLS delivery, JWT-secured playback, and a live Quality of Experience (QoE) dashboard.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Prerequisites](#2-prerequisites)
3. [Quick Start](#3-quick-start)
4. [Configuration](#4-configuration)
5. [Directory Structure](#5-directory-structure)
6. [Database Schema](#6-database-schema)
7. [HTTP API Reference](#7-http-api-reference)
8. [Authentication & Authorization](#8-authentication--authorization)
9. [VOD Pipeline](#9-vod-pipeline)
10. [Live Streaming Pipeline](#10-live-streaming-pipeline)
11. [QoE Aggregator](#11-qoe-aggregator)
12. [Frontend Templates](#12-frontend-templates)
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
                    ┌─────────────────────────────────────────────────────┐
                    │                   Go HTTP Server (:8080)             │
                    │                                                       │
  Browser ──────────┤  Pages         (/,/upload,/dashboard,/watch/…)      │
                    │  Video API     (/api/v1/videos/…)                    │
  OBS/Encoder ──────┤  Upload API    (/api/v1/uploads/…)                  │
     RTMP :1935     │  Session API   (/api/v1/…/sessions)                 │
                    │  Telemetry     (/api/v1/sessions/…/events)           │
                    │  Dashboard SSE (/api/v1/dashboard/stats/stream)      │
                    │  HLS serving   (/videos/…  /live/…)  JWT protected   │
                    └───────────────┬──────────────────────────────────────┘
                                    │
                    ┌───────────────▼────────────────┐
                    │         PostgreSQL 15           │
                    │  videos, upload_chunks,         │
                    │  transcode_jobs,                │
                    │  playback_sessions/events,      │
                    │  stream_keys, live_streams      │
                    └────────────────────────────────┘

VOD path:  Upload → assemble → TranscodeWorker → FFmpeg → HLS fMP4 → data/hls/
Live path: OBS → RTMP → go-rtmp → FLV pipe → FFmpeg → HLS TS → data/live/
```

### Component layers

| Layer | Purpose |
|---|---|
| `cmd/server` | Wire-up: config → DB → repos → services → workers → handlers → routes |
| `internal/handler` | HTTP request parsing, response encoding |
| `internal/service` | Business logic (no HTTP, no SQL) |
| `internal/repository` | SQL queries, returns model structs |
| `internal/live` | RTMP server + per-stream FFmpeg session management |
| `internal/transcoder` | Low-level FFmpeg/FFprobe wrappers for VOD |
| `internal/qoe` | In-memory sliding-window metrics aggregation |
| `internal/middleware` | JWT auth, applied at route registration |

---

## 2. Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.22+ | Build & run |
| FFmpeg + FFprobe | Any modern | Video encoding (must be in `$PATH`) |
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
# 1. Start PostgreSQL
make db

# 2. Start the server (runs migrations automatically)
make serve

# 3. Open the browser
open http://localhost:8080
```

The server automatically runs all database migrations on startup — no manual SQL required.

### Makefile targets

| Target | Command | Description |
|---|---|---|
| `make db` | `docker compose up -d postgres` | Start PostgreSQL in Docker |
| `make stop` | `docker compose down` | Stop PostgreSQL |
| `make serve` | `go run ./cmd/server` | Run HTTP + RTMP server |
| `make dev` | `air` or `go run ./cmd/server` | Live-reload dev server |
| `make build` | Compiles to `bin/` | Build production binaries |
| `make transcode INPUT=… OUTPUT=…` | CLI batch transcode | Transcode without the web server |
| `make clean` | Remove `bin/` and `data/` | Clean build artifacts |

---

## 4. Configuration

All configuration is read from environment variables. Default values are safe for local development.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://philos:philos@localhost:5433/philos_video?sslmode=disable` | PostgreSQL DSN |
| `DATA_DIR` | `./data` | Root for all video storage (chunks, raw, hls, live) |
| `WORKER_COUNT` | `2` | Concurrent transcode workers |
| `JWT_SECRET` | `dev-secret-…` | **Must be changed in production.** Min 32 chars. |
| `JWT_EXPIRY` | `1h` | Playback token lifetime (Go duration format: `30m`, `2h`, etc.) |
| `RTMP_PORT` | `1935` | RTMP ingest port |

**Production example:**

```bash
export PORT=8080
export DATABASE_URL="postgres://user:pass@db-host:5432/philos_video?sslmode=require"
export DATA_DIR="/mnt/video-storage"
export WORKER_COUNT=4
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_EXPIRY="1h"
export RTMP_PORT=1935
./bin/server
```

---

## 5. Directory Structure

```
philos-video/
├── cmd/
│   ├── server/main.go          # Entry point: wires all components, registers routes
│   └── transcode/main.go       # Standalone CLI for batch transcoding
│
├── internal/
│   ├── config/config.go        # Env var parsing → Config struct
│   ├── database/postgres.go    # sql.Open + Migrate (inlined SQL)
│   │
│   ├── models/models.go        # All DB-facing structs + status constants
│   │
│   ├── repository/             # One file per DB table
│   │   ├── video_repo.go
│   │   ├── upload_repo.go
│   │   ├── job_repo.go
│   │   ├── session_repo.go
│   │   ├── event_repo.go       # Also defines nil-helper functions ns/ni/ni64/nf64
│   │   ├── stream_key_repo.go
│   │   └── live_stream_repo.go
│   │
│   ├── service/                # Business logic (no HTTP, no SQL)
│   │   ├── video_service.go
│   │   ├── upload_service.go
│   │   ├── transcode_service.go
│   │   └── session_service.go  # JWT generation + PlaybackClaims struct
│   │
│   ├── handler/                # HTTP handlers (parse request → call service → encode response)
│   │   ├── upload_handler.go
│   │   ├── video_handler.go
│   │   ├── session_handler.go
│   │   ├── telemetry_handler.go
│   │   ├── dashboard_handler.go
│   │   ├── stream_key_handler.go
│   │   ├── live_handler.go
│   │   └── page_handler.go     # HTML template rendering
│   │
│   ├── middleware/auth.go      # JWT validation middleware (VOD + live)
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
│   │
│   ├── qoe/aggregator.go       # In-memory 5-min sliding window QoE metrics
│   │
│   └── web/
│       ├── embed.go            # //go:embed templates → web.Templates
│       └── templates/
│           ├── library.html    # Video library + live section
│           ├── upload.html     # Chunked upload UI
│           ├── player.html     # VOD HLS.js player + TelemetryClient
│           ├── dashboard.html  # Real-time QoE dashboard (SSE)
│           ├── go_live.html    # Stream key management + OBS guide
│           └── watch_live.html # Live HLS.js player
│
├── migrations/                 # Documentation-only SQL (inlined in database/postgres.go)
│   ├── 001_initial.sql
│   ├── 002_sessions_and_events.sql
│   └── 003_live_streaming.sql
│
├── data/                       # Runtime-generated, gitignored
│   ├── chunks/{upload_id}/     # Raw uploaded chunks (deleted after assembly)
│   ├── raw/{upload_id}/        # Assembled input file (deleted after transcode)
│   ├── hls/{video_id}/         # Final VOD output served at /videos/{id}/…
│   └── live/{stream_id}/       # Live HLS output served at /live/{id}/…
│
├── go.mod / go.sum
├── Makefile
└── docker-compose.yml          # postgres:15 on port 5433
```

### Data directory layout (at runtime)

```
data/
├── chunks/
│   └── {upload_id}/
│       ├── 00000           ← raw chunk bytes
│       ├── 00001
│       └── ...
├── raw/
│   └── {upload_id}/
│       └── original.mp4    ← assembled file (deleted after transcode)
├── hls/
│   └── {video_id}/
│       ├── master.m3u8     ← served at /videos/{video_id}/master.m3u8
│       ├── 720p/
│       │   ├── playlist.m3u8
│       │   ├── init.mp4    ← fMP4 init segment
│       │   └── segment_0000.m4s ...
│       ├── 480p/ ...
│       └── 360p/ ...
└── live/
    └── {stream_id}/
        ├── master.m3u8     ← pre-written at stream start
        ├── 720p/
        │   ├── playlist.m3u8  ← sliding window (5 segments)
        │   └── segment_0000.ts ...
        ├── 480p/ ...
        └── 360p/ ...
```

---

## 6. Database Schema

Migrations run automatically on startup via `database.Migrate(db)`. The SQL is inlined in `internal/database/postgres.go` (so `go:embed` can reach it from within the module).

### `videos`
Primary record for a video asset. Created at upload, updated through the transcode pipeline.

```sql
id         TEXT PRIMARY KEY              -- same as upload_id (e.g. random hex)
title      TEXT NOT NULL                 -- original filename or user-provided title
status     TEXT NOT NULL DEFAULT 'uploading'
           -- uploading → processing → ready | failed
width      INT                           -- set after probe
height     INT
duration   TEXT                          -- e.g. "00:03:47.00"
codec      TEXT                          -- e.g. "h264"
hls_path   TEXT                          -- relative path from DATA_DIR
           -- VOD:  "hls/{video_id}"
           -- Live recording: "live/{stream_id}"
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `upload_chunks`
Tracks which chunks of a chunked upload have been received.

```sql
upload_id    TEXT                        -- matches video.id
chunk_number INT
received     BOOLEAN NOT NULL DEFAULT FALSE
PRIMARY KEY (upload_id, chunk_number)
```

### `transcode_jobs`
One job per video, tracks FFmpeg progress.

```sql
id         TEXT PRIMARY KEY
video_id   TEXT NOT NULL REFERENCES videos(id)
status     TEXT NOT NULL DEFAULT 'queued'
           -- queued → running → completed | failed
stage      TEXT                         -- current FFmpeg stage name
progress   DOUBLE PRECISION DEFAULT 0   -- 0.0–1.0
error      TEXT                         -- error message if failed
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

**Progress milestones:** `probe=0.05`, `prepare=0.10`, `encode per profile 0.10–0.80`, `segment=0.85`, `packaging=0.95`, `done=1.0`

### `playback_sessions`
Created when a viewer clicks play. Stores the JWT token and tracks activity.

```sql
id             TEXT PRIMARY KEY          -- sess_{hex}
video_id       TEXT REFERENCES videos(id) -- nullable (null for live sessions)
stream_id      TEXT                      -- set for live sessions
token          TEXT NOT NULL             -- the JWT string
device_type    TEXT                      -- mobile | tablet | desktop
user_agent     TEXT
ip_address     TEXT
started_at     TIMESTAMPTZ DEFAULT NOW()
last_active_at TIMESTAMPTZ DEFAULT NOW() -- debounced: updated at most every 30s
ended_at       TIMESTAMPTZ               -- set on playback_end event
status         TEXT DEFAULT 'active'     -- active | ended
```

### `playback_events`
Time-series event log from each player. High-volume; indexed for range queries.

```sql
id                   BIGSERIAL PRIMARY KEY
session_id           TEXT NOT NULL REFERENCES playback_sessions(id)
video_id             TEXT NOT NULL
event_type           TEXT NOT NULL
  -- playback_start, segment_downloaded, quality_change,
  -- rebuffer_start, rebuffer_end, heartbeat, playback_end
timestamp            TIMESTAMPTZ DEFAULT NOW()
segment_number       INTEGER
segment_quality      TEXT         -- 720p | 480p | 360p
segment_bytes        BIGINT
download_time_ms     INTEGER
throughput_bps       BIGINT       -- bytes*8 / download_time_seconds
current_quality      TEXT         -- from heartbeat
buffer_length        DOUBLE PRECISION  -- seconds ahead
playback_position    DOUBLE PRECISION  -- current time in video
rebuffer_duration_ms INTEGER
quality_from         TEXT
quality_to           TEXT
error_code           TEXT
error_message        TEXT

-- Indexes:
--   idx_playback_events_session   ON (session_id)
--   idx_playback_events_type_time ON (event_type, timestamp)
--   idx_playback_events_video     ON (video_id, timestamp)
```

### `stream_keys`
Credentials for RTMP publishing. OBS uses the `id` as the stream key.

```sql
id         TEXT PRIMARY KEY   -- sk_{8 random hex bytes}
user_label TEXT NOT NULL      -- human-readable name
is_active  BOOLEAN DEFAULT TRUE
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `live_streams`
One record per RTMP ingest session.

```sql
id            TEXT PRIMARY KEY      -- ls_{8 random hex bytes}
stream_key_id TEXT NOT NULL REFERENCES stream_keys(id)
title         TEXT NOT NULL         -- defaults to stream_key.user_label
status        TEXT DEFAULT 'waiting'
              -- waiting → live → ended
source_width  INT                   -- set when RTMP announces resolution
source_height INT
source_codec  TEXT
source_fps    TEXT
hls_path      TEXT                  -- "live/{stream_id}" (relative to DATA_DIR)
video_id      TEXT                  -- set when VOD recording is created on stream end
started_at    TIMESTAMPTZ
ended_at      TIMESTAMPTZ
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- Indexes:
--   idx_live_streams_status     ON (status)
--   idx_live_streams_stream_key ON (stream_key_id)
```

---

## 7. HTTP API Reference

### Upload

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/api/v1/uploads` | `upload.InitUpload` | Start a new chunked upload |
| `PUT` | `/api/v1/uploads/{upload_id}/chunks/{chunk_number}` | `upload.ReceiveChunk` | Send one chunk (raw body) |
| `GET` | `/api/v1/uploads/{upload_id}/status` | `upload.GetStatus` | `{"received":3,"total":5}` |

**InitUpload request body:**
```json
{ "filename": "video.mp4", "total_chunks": 5 }
```
**InitUpload response:**
```json
{ "upload_id": "a1b2c3d4e5f6..." }
```

### Videos

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/api/v1/videos` | `video.ListVideos` | Array of all videos (desc by `created_at`) |
| `GET` | `/api/v1/videos/{id}` | `video.GetVideo` | Single video record |
| `GET` | `/api/v1/videos/{id}/status` | `video.GetVideoStatus` | Video + job + progress (`0.0–1.0`) |

**GetVideoStatus response:**
```json
{
  "video": { "id": "…", "title": "…", "status": "processing", … },
  "job":   { "id": "…", "status": "running", "stage": "encode:720p", "progress": 0.35 },
  "progress": 0.35
}
```

### Playback Sessions (VOD)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/api/v1/videos/{id}/sessions` | `session.CreateSession` | Validate video is ready, create JWT-backed session |

**Request body:**
```json
{ "device_type": "desktop", "user_agent": "optional override" }
```
**Response:**
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

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/api/v1/sessions/{session_id}/events` | `telemetry.PostEvents` | Batch ingest playback events |

**Request body:**
```json
{
  "events": [
    {
      "event_type": "heartbeat",
      "timestamp": "2025-03-10T12:00:00Z",
      "current_quality": "720p",
      "buffer_length": 8.5,
      "playback_position": 45.2
    },
    {
      "event_type": "segment_downloaded",
      "segment_number": 12,
      "segment_quality": "720p",
      "segment_bytes": 524288,
      "download_time_ms": 210,
      "throughput_bps": 19999390
    }
  ]
}
```

**Event types and their fields:**

| event_type | Key fields |
|---|---|
| `playback_start` | `download_time_ms` (TTFF), `buffer_length` |
| `segment_downloaded` | `segment_number`, `segment_quality`, `segment_bytes`, `download_time_ms`, `throughput_bps` |
| `quality_change` | `quality_from`, `quality_to` |
| `rebuffer_start` | *(no extra fields)* |
| `rebuffer_end` | `rebuffer_duration_ms` |
| `heartbeat` | `current_quality`, `buffer_length`, `playback_position` |
| `playback_end` | *(marks session ended)* |

### Dashboard

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/api/v1/dashboard/stats` | `dashboard.GetStats` | One-shot JSON metrics snapshot |
| `GET` | `/api/v1/dashboard/stats/stream` | `dashboard.StatsStream` | SSE stream (`text/event-stream`), 1 update/second |

**DashboardMetrics fields:**

| Field | Type | Description |
|---|---|---|
| `timestamp` | RFC3339 | When snapshot was computed |
| `active_sessions` | int | Sessions with heartbeat in last 60s |
| `total_sessions_5m` | int | Unique sessions in 5-minute window |
| `ttff_median_ms` | int | Median Time-To-First-Frame |
| `ttff_p95_ms` | int | p95 Time-To-First-Frame |
| `rebuffer_rate` | float | Fraction of sessions with a rebuffer (0.0–1.0) |
| `avg_rebuffer_duration_ms` | int | Average rebuffer duration |
| `avg_bitrate_kbps` | float | Quality-weighted average bitrate |
| `quality_distribution` | map | `{"720p":0.6,"480p":0.3,"360p":0.1}` |
| `quality_switches_per_min` | float | Quality changes / 5 minutes |
| `avg_throughput_mbps` | float | Mean segment download speed |
| `p10_throughput_mbps` | float | Worst 10th percentile throughput |
| `per_video` | array | Top 10 videos by active sessions |
| `active_live_streams` | int | Currently transcoding RTMP streams |

### Stream Keys

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/api/v1/stream-keys` | `streamkey.Create` | Create new key `{"label":"…"}` |
| `GET` | `/api/v1/stream-keys` | `streamkey.List` | List all keys |
| `DELETE` | `/api/v1/stream-keys/{id}` | `streamkey.Deactivate` | Revoke a key (`204 No Content`) |

### Live Streams

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/api/v1/live` | `live.ListLive` | All streams with `status=live` |
| `GET` | `/api/v1/live/{stream_id}` | `live.GetStream` | Single live stream record |
| `POST` | `/api/v1/live/{stream_id}/sessions` | `live.CreateSession` | JWT session for HLS access |
| `POST` | `/api/v1/live/{stream_id}/end` | `live.EndStream` | Manually end stream + trigger VOD |

### File Serving (JWT-Protected)

| Pattern | Auth check | Serves from |
|---|---|---|
| `GET /videos/{id}/…` | JWT `vid` claim == `{id}` | `{DATA_DIR}/hls/{id}/` |
| `GET /live/{id}/…` | JWT `stid` claim == `{id}` | `{DATA_DIR}/live/{id}/` |

Token must be passed as `?token=<jwt>` query parameter on every request. Live routes also send `Cache-Control: no-cache, no-store`.

---

## 8. Authentication & Authorization

### JWT Token Structure

All playback tokens are HS256-signed JWTs. Claims:

```json
{
  "jti":  "sess_abc123",    // JWT ID = session ID
  "iat":  1710000000,
  "exp":  1710003600,
  "sid":  "sess_abc123",    // session ID (always present)
  "vid":  "a1b2c3…",        // video ID (VOD only)
  "stid": "ls_d4e5f6…"      // stream ID (live only)
}
```

### Auth Flow

```
Client:  POST /api/v1/videos/{id}/sessions
Server:  Validate video.status == "ready"
         Create PlaybackSession record
         Sign JWT with {sid, vid} + expiry
         Return token + manifest URL

Client:  GET /videos/{id}/master.m3u8?token=<jwt>
Middleware: Parse JWT → extract claims
            Check claims.VideoID == path ID
            Touch session.last_active_at (debounced 30s)
            Pass to file server
```

The same flow applies to live streams using `stid` and `/live/{stream_id}/`.

### Middleware (`internal/middleware/auth.go`)

```go
// For VOD routes:
mux.Handle("GET /videos/", authMiddleware.RequirePlaybackToken(hlsHandler))

// For live routes:
mux.Handle("GET /live/", authMiddleware.RequireLiveToken(liveHLSHandler))
```

Both methods validate the JWT, extract the resource ID from the URL path, and compare it against the claim.

---

## 9. VOD Pipeline

### Upload Phase

1. Client calls `POST /api/v1/uploads` → server creates `videos` and `upload_chunks` records, returns `upload_id`.
2. Client splits file into 5 MB chunks, sends each via `PUT /api/v1/uploads/{id}/chunks/{n}`.
3. Each chunk is written to `data/chunks/{upload_id}/{n}`.
4. When last chunk lands, `UploadService.assemble()` runs in a goroutine:
   - Concatenates all chunks in order → `data/raw/{upload_id}/original.{ext}`
   - Deletes chunk files
   - Creates `TranscodeJob` record (status=queued)
   - Sends `job_id` to the worker channel

### Transcode Phase

The `TranscodeWorker` goroutine pool reads from the job channel:

1. **Probe** (`internal/transcoder/probe.go`): `ffprobe -print_format json -show_streams -show_format` → extracts width, height, codec, duration.
2. **Build Ladder** (`internal/transcoder/ladder.go`): filters the 3-profile ladder to only include resolutions ≤ source.
3. **Per profile** (`internal/transcoder/encode.go` + `segment.go`):
   - Encode: `ffmpeg … -c:v libx264 -preset medium -movflags +faststart` → `data/hls/{id}/{profile}/intermediate.mp4`
   - Segment: `ffmpeg -c copy -f hls -hls_segment_type fmp4 -hls_time 4` → `data/hls/{id}/{profile}/playlist.m3u8` + segments
   - Delete `intermediate.mp4`
4. **Master Manifest** (`internal/transcoder/manifest.go`): `data/hls/{id}/master.m3u8`
5. Update `videos.hls_path`, `videos.status = ready`, job status = completed.

### Encoding Ladder

| Profile | Resolution | Video bitrate | Audio | MaxRate |
|---------|-----------|--------------|-------|---------|
| 720p | 1280×720 | 2500 kbps | 128 kbps | 2500 kbps |
| 480p | 854×480 | 1000 kbps | 96 kbps | 1000 kbps |
| 360p | 640×360 | 400 kbps | 64 kbps | 400 kbps |

Profiles whose height exceeds the source height are skipped. E.g., a 480p source only gets the 480p and 360p profiles.

### Serving VOD

The `hls/` directory is served by `http.FileServer` with:
- MIME types set by `mimeHandler`: `.m3u8 → application/vnd.apple.mpegurl`, `.m4s → video/iso.bmff`, `.mp4 → video/mp4`
- JWT validation via `RequirePlaybackToken` middleware

---

## 10. Live Streaming Pipeline

### Broadcaster Setup (OBS)

1. Create a stream key at `http://localhost:8080/go-live` → copy the `sk_…` ID.
2. In OBS: **Settings → Stream → Service: Custom → Server: `rtmp://localhost:1935/live`**
3. Set Stream Key to the `sk_…` value.
4. Start Streaming.

### RTMP Ingest Path

```
OBS ──RTMP──▶ RTMPServer (internal/live/rtmp_server.go)
                 └─ go-rtmp Server.Serve(net.Listener)
                      └─ per connection: rtmpHandler
                           OnPublish() → Manager.StartStream(streamKey)
                             1. Validate stream key in DB
                             2. Create live_streams record (status=live)
                             3. newTranscodeSession() → spawn FFmpeg
                             4. Write FLV file header to FFmpeg stdin
                           OnVideo(timestamp, payload) → writeTag(0x09, …)
                           OnAudio(timestamp, payload) → writeTag(0x08, …)
                           OnClose() → Manager.EndStream()
```

### FLV Framing

RTMP message payloads are the raw FLV tag data bytes. `transcodeSession.writeTag()` wraps each payload with a proper FLV tag header (11 bytes) and appends the `PreviousTagSize` (4 bytes):

```
FLV file header (13 bytes):
  "FLV" + version=0x01 + flags=0x05 + data_offset=9 + PreviousTagSize0=0

Per tag (video=0x09, audio=0x08):
  TagType  (1 byte)
  DataSize (3 bytes, big-endian)
  Timestamp lower 24 bits (3 bytes, big-endian)
  TimestampExtended upper 8 bits (1 byte)
  StreamID (3 bytes, always 0)
  Data     (DataSize bytes) ← RTMP payload
  PreviousTagSize (4 bytes, big-endian) = DataSize + 11
```

### Live FFmpeg Pipeline

```
FFmpeg stdin (FLV stream)
  ↓
-f flv -i pipe:0
  ↓
-filter_complex "[0:v]split=3[raw720][raw480][raw360];
                  [raw720]scale=1280:720[v720];
                  [raw480]scale=854:480[v480];
                  [raw360]scale=640:360[v360]"
  ↓
3 video streams + 3 audio streams
  ↓
-f hls
  -hls_time 2                   (2-second segments — lower latency than VOD)
  -hls_list_size 5              (sliding window: keep only 5 segments)
  -hls_flags delete_segments+independent_segments+append_list
  -hls_segment_type mpegts      (MPEG-TS segments; simpler than fMP4 for live)
  -var_stream_map "v:0,a:0,name:720p v:1,a:1,name:480p v:2,a:2,name:360p"
  ↓
data/live/{stream_id}/720p/playlist.m3u8  (+ segment_0000.ts … segment_0004.ts)
data/live/{stream_id}/480p/playlist.m3u8
data/live/{stream_id}/360p/playlist.m3u8
data/live/{stream_id}/master.m3u8   ← written upfront (fixed content)
```

**Live codec settings:**

| Stream | Video codec | Preset | Tune | GOP |
|--------|-------------|--------|------|-----|
| 720p | libx264 | veryfast | zerolatency | 60 |
| 480p | libx264 | veryfast | zerolatency | 60 |
| 360p | libx264 | veryfast | zerolatency | 60 |

`-tune zerolatency` disables B-frames and delays to minimize end-to-end latency (~2–4 seconds from OBS to viewer).

### Stream End & VOD Conversion

When OBS disconnects or `POST /api/v1/live/{id}/end` is called:

1. `Manager.EndStream(streamID)`:
   - Closes FFmpeg stdin → FFmpeg flushes + exits
   - Appends `#EXT-X-ENDLIST` to all variant playlists (makes VOD-seekable)
   - Updates `live_streams.status = ended`, `ended_at = NOW()`
2. `Manager.convertToVOD(streamID)` (goroutine):
   - Creates a `videos` record with `status=ready`, `hls_path=live/{stream_id}`
   - Calls `videoRepo.UpdateHLSPath()` → video immediately playable via `/videos/{video_id}`
   - Stores `video_id` in `live_streams` record for linking

---

## 11. QoE Aggregator

**File:** `internal/qoe/aggregator.go`

### Design

The aggregator runs entirely in memory. No database reads happen during metrics calculation — the telemetry handler writes to both DB and the aggregator. This keeps the dashboard SSE latency well under 1 second.

```
┌─────────────────────────────────────────┐
│  Aggregator                             │
│                                         │
│  recentEvents  []timedEvent             │  ← 5-min sliding window
│  activeSessions map[session_id]time     │  ← last heartbeat
│  sessionToVideo map[session_id]video_id │
│  videoTitles   map[video_id]string      │  ← lazy-fetched from DB
│                                         │
│  Background goroutine:                  │
│   every 1s  → recalculate() + broadcast │
│   every 10s → pruneEvents()            │
│   every 30s → pruneSessions()          │
└─────────────────────────────────────────┘
```

### Metric Calculations

| Metric | Method |
|---|---|
| Active sessions | Sessions with heartbeat timestamp < 60s ago |
| TTFF | `playback_start.download_time_ms` values → percentile(50) and percentile(95) |
| Rebuffer rate | `len(sessions with rebuffer_start) / len(all sessions)` |
| Avg rebuffer duration | Mean of `rebuffer_end.rebuffer_duration_ms` values |
| Quality distribution | Count of each quality in heartbeats / total heartbeats |
| Avg bitrate | Σ(quality_fraction × quality_bitrate_kbps) |
| Quality switches/min | `count(quality_change events) / 5` |
| Throughput | Mean and p10 of `segment_downloaded.throughput_bps` / 1_000_000 |

### SSE Subscription Pattern

```go
// Dashboard handler subscribes on connect, unsubscribes on disconnect
ch := aggregator.Subscribe()    // buffered chan (capacity 2)
defer aggregator.Unsubscribe(ch)
for metrics := range ch {
    fmt.Fprintf(w, "data: %s\n\n", json.Marshal(metrics))
}
```

If the subscriber is slow, the aggregator drops the update (non-blocking send).

### Live Counter Integration

The `live.Manager` implements the `qoe.LiveCounter` interface:
```go
type LiveCounter interface {
    ActiveCount() int   // returns len(sessions)
}
```

Wired in `cmd/server/main.go`:
```go
aggregator.SetLiveCounter(liveMgr)
```

---

## 12. Frontend Templates

All templates are embedded at compile time via `internal/web/embed.go`. The `PageHandler` parses them at startup using `template.ParseFS(web.Templates, "templates/*.html")`.

### TelemetryClient (in `player.html`)

A JavaScript class that buffers playback events and flushes them every 3 seconds:

```js
class TelemetryClient {
  constructor(sessionId, telemetryUrl) { … }

  // Called by hls.js events:
  recordPlaybackStart(bufferLength)
  recordSegmentDownloaded(fragData)
  recordQualityChange(fromLevel, toLevel)
  recordRebufferStart()
  recordRebufferEnd()

  // Called by setInterval(5000):
  recordHeartbeat(video, hls)

  // Batches events, flushes every 3s to telemetry_url:
  push(event)
  flush()          // POST {events:[…]} to telemetryUrl
}
```

### Library page auto-refresh

The library polls `/api/v1/videos` every 3 seconds and `/api/v1/live` every 5 seconds. Cards are updated in-place (only re-rendered if status changed or currently processing), avoiding full-page refreshes.

### Upload chunking logic (`upload.html`)

```
File selected
  → POST /api/v1/uploads { filename, total_chunks }
  → For chunk 0 .. n-1:
       PUT /api/v1/uploads/{id}/chunks/{n}  (5 MB slice)
  → Poll /api/v1/videos/{id}/status until status != "processing"
  → Redirect to /watch/{id}
```

Upload progress is shown as two phases: upload (0–70%) and processing (70–100% mapped from job.progress).

---

## 13. Internal Packages

### `internal/repository`

One struct per table, all methods take `context.Context` (except a few sync ones). All SQL uses `$N` positional parameters (PostgreSQL style).

Nullable string/int helpers are defined in `event_repo.go` and used across the package:
```go
func ns(s string) interface{}  { if s == "" { return nil }; return s }
func ni(n *int) interface{}    { if n == nil { return nil }; return *n }
func ni64(n *int64) interface{} { … }
func nf64(n *float64) interface{} { … }
```

### `internal/service`

Pure business logic. No HTTP types, no `sql.DB`. Receives repos/dependencies via constructor injection.

### `internal/live`

The `Manager` is the single owner of all active `transcodeSession` objects. The `rtmpHandler` (one per TCP connection) holds a reference to `Manager` and calls `StartStream` / `WriteVideo` / `WriteAudio` / `EndStream`. No direct repo access from `rtmpHandler`.

### `internal/transcoder`

Pure FFmpeg/FFprobe wrappers. Used only by `TranscodeService`. Can also be used directly by the `cmd/transcode` CLI.

---

## 14. Data Flow Diagrams

### VOD Upload → Playback

```
Browser                 UploadService           DB              Worker
   │                        │                    │                │
   │─ POST /api/v1/uploads ─▶│ create video+chunks│                │
   │◀─ {upload_id} ─────────│                    │                │
   │                        │                    │                │
   │─ PUT …/chunks/0 ───────▶│ save to disk       │                │
   │─ PUT …/chunks/1 ───────▶│ mark received      │                │
   │─ PUT …/chunks/N ───────▶│ last chunk!        │                │
   │                        │─ assemble() ───────▶│ create job     │
   │                        │                    │──── jobCh ────▶│
   │                        │                    │                │ Process()
   │                        │                    │◀─ update prog─ │
   │─ GET /api/v1/videos/{id}/status ──────────────▶│              │
   │◀─ {status:"processing",progress:0.35} ─────────│              │
   │                        │                    │◀─ status=ready─│
   │─ GET /watch/{id}       │                    │                │
   │─ POST …/sessions ──────────────────────────▶│ create session │
   │◀─ {manifest_url,token} ────────────────────│                │
   │─ GET /videos/{id}/master.m3u8?token=… ─────── auth check ──▶│
   │◀─ HLS manifest ──────────────────────────────────────────────│
```

### Live Broadcast → Viewer

```
OBS              RTMPHandler          Manager              FFmpeg (child proc)
 │                    │                  │                       │
 │─ RTMP connect ────▶│                  │                       │
 │─ RTMP publish ────▶│ OnPublish()      │                       │
 │  key=sk_…          │─ StartStream() ─▶│ newTranscodeSession() │
 │                    │                  │─ exec ffmpeg ────────▶│
 │                    │                  │─ write FLV header ────▶│
 │─ video packet ────▶│ OnVideo()        │                       │
 │                    │─ WriteVideo() ───▶│─ writeTag(0x09) ─────▶│
 │─ audio packet ────▶│ OnAudio()        │                       │
 │                    │─ WriteAudio() ───▶│─ writeTag(0x08) ─────▶│
 │                    │                  │                  produces HLS
 │                    │                  │               data/live/{id}/
 │─ RTMP disconnect ─▶│ OnClose()        │                       │
 │                    │─ EndStream() ────▶│ close stdin ─────────▶│
 │                    │                  │   ← ffmpeg exits       │
 │                    │                  │ append #EXT-X-ENDLIST  │
 │                    │                  │ create VOD video record │

Viewer               AuthMiddleware       LiveHandler
 │                       │                   │
 │─ POST /api/v1/live/{id}/sessions ─────────▶│ create session + JWT
 │◀─ {manifest_url,token} ───────────────────│
 │─ GET /live/{id}/master.m3u8?token=… ──────── verify stid claim
 │◀─ HLS manifest (no-cache) ──────────────────────────────────────
 │─ GET /live/{id}/720p/playlist.m3u8?… ───────── (repeated ~every 2s)
```

---

## 15. Adding Features

### Adding a new API endpoint

1. Add SQL if needed to `internal/database/postgres.go` (append to `migrationSQL`)
2. Add model struct/constants to `internal/models/models.go`
3. Add repository method to the relevant `internal/repository/*.go`
4. Add service method to `internal/service/*.go`
5. Create handler method in `internal/handler/*.go`
6. Register route in `cmd/server/main.go`

### Adding a new video quality profile

Edit `internal/transcoder/ladder.go`. Add a `Profile` struct to the `defaultLadder` slice. Also update:
- `qualityBitrates` map in `internal/qoe/aggregator.go` for accurate bitrate weighting
- The live FFmpeg args in `internal/live/transcode_session.go`
- The `liveMasterPlaylist` constant in the same file
- The quality bars in `internal/web/templates/dashboard.html`

### Adding a new telemetry event type

1. If it needs new fields, add columns to `playback_events` in the migration SQL.
2. Add the field to `models.PlaybackEvent`.
3. Update `event_repo.go`'s `BatchInsert` (increment `numCols`, add the new column).
4. Add the recording call in the browser (`player.html`'s `TelemetryClient`).
5. Handle it in `aggregator.go`'s `recalculate()` if it affects dashboard metrics.

### Changing the JWT expiry behaviour

`JWT_EXPIRY` is a Go duration string. The token is included in the HLS manifest URL (as a query parameter). If the token expires during a viewing session, segment requests will fail. For long videos, consider:
- Setting `JWT_EXPIRY` to a long duration (e.g., `12h`)
- Or implementing token refresh (not currently implemented)

---

## 16. Deployment

### Build production binaries

```bash
make build
# Produces: bin/server, bin/transcode
```

### Docker Compose for PostgreSQL

```yaml
# docker-compose.yml (provided)
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

The server port is **5433** (not 5432) to avoid conflicts with local Postgres installs.

### Systemd service example

```ini
[Unit]
Description=PramTube Server
After=network.target postgresql.service

[Service]
User=www-data
WorkingDirectory=/opt/philos-video
ExecStart=/opt/philos-video/bin/server
Environment=PORT=8080
Environment=DATABASE_URL=postgres://…
Environment=DATA_DIR=/var/lib/philos-video/data
Environment=WORKER_COUNT=4
Environment=JWT_SECRET=<generated>
Environment=RTMP_PORT=1935
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

### Reverse proxy (nginx) example

```nginx
server {
    listen 80;
    server_name video.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection keep-alive;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        # Disable buffering for SSE (dashboard stream)
        proxy_buffering off;
        proxy_cache off;
    }
}

# RTMP is a TCP protocol; use stream {} block for proxying if needed
stream {
    server {
        listen 1935;
        proxy_pass 127.0.0.1:1935;
    }
}
```

---

## 17. Security Considerations

| Area | Current State | Recommendation |
|---|---|---|
| `JWT_SECRET` | Dev default in code | **Must set a strong secret in production** (32+ random chars) |
| Stream keys | Stored as plaintext `sk_*` IDs | Consider hashing with HMAC before DB storage |
| Stream key API | Publicly accessible | Add admin authentication (basic auth or session-based) |
| Rate limiting | None | Add per-IP rate limits on upload init, session create, event ingest |
| CORS | None configured | Add `Access-Control-Allow-Origin` if frontend moves to separate origin |
| HLS token binding | Token includes resource ID | Prevents using a VOD token for a different video or live stream |
| `last_active_at` | Debounced 30s | Reduces DB load; 30s window means stale sessions linger briefly |
| Input sanitation | Filenames stored as-is | File extension validation already done; consider length limits |
| SQL injection | Parameterized queries (`$N`) | Protected |
| FFmpeg injection | Args built from DB values | User-controlled values (title) are not passed to FFmpeg args |

---

## 18. Known Limitations

1. **Safari native HLS**: When HLS.js isn't supported, the player falls back to the `<video>` element's native HLS. The JWT token query parameter will work for the master playlist but **not** for sub-playlists fetched natively by Safari — HLS.js's `xhrSetup` cannot intercept native requests. This is noted in `player.html`.

2. **Live stream audio assumption**: The live FFmpeg pipeline assumes the RTMP stream contains at least one audio track. Video-only streams will cause FFmpeg to fail. If OBS is sending video-only, remove the `-map 0:a` lines in `transcode_session.go` and adjust `var_stream_map` accordingly.

3. **No token refresh**: Playback tokens expire based on `JWT_EXPIRY`. For long videos or live streams, set a generous expiry. Token renewal is not implemented.

4. **Single database migration**: All migrations run as one idempotent SQL block (`CREATE TABLE IF NOT EXISTS`, `ALTER TABLE … ADD COLUMN IF NOT EXISTS`). There is no migration versioning system. Schema changes require care to keep them idempotent.

5. **In-memory live sessions**: Active transcode sessions are stored in `live.Manager`'s `sessions` map. Server restart terminates all live streams. The `live_streams` records would be stuck in `status=live` — run `UPDATE live_streams SET status='ended' WHERE status='live'` after an unclean shutdown.

6. **No CDN integration**: HLS files are served directly from the Go process. For production traffic, point a CDN (CloudFront, Fastly) at the `/videos/` and `/live/` routes, or serve the `data/hls/` and `data/live/` directories from an object store (S3, GCS).

7. **No user accounts**: The platform has no user authentication. All APIs are accessible to anyone on the network. The stream key system provides minimal broadcaster auth; viewer access is controlled only by JWT tokens.

8. **Data directory must be local**: The transcode workers and live FFmpeg processes write to `DATA_DIR` on the local filesystem. Shared network storage (NFS, etc.) is possible but untested and may have performance issues.

---

## 19. Dependencies

### Go modules

| Module | Version | Purpose |
|---|---|---|
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT token signing and validation |
| `github.com/lib/pq` | v1.11.2 | PostgreSQL driver |
| `github.com/yutopp/go-rtmp` | v0.0.7 | RTMP server implementation |
| `github.com/yutopp/go-amf0` | v0.1.0 | AMF0 serialization (go-rtmp dependency) |
| `github.com/hashicorp/go-multierror` | v1.1.0 | Multi-error aggregation (go-rtmp dependency) |
| `github.com/mitchellh/mapstructure` | v1.4.1 | Struct mapping (go-rtmp dependency) |
| `github.com/pkg/errors` | v0.9.1 | Error wrapping (go-rtmp dependency) |
| `github.com/sirupsen/logrus` | v1.7.0 | Structured logging (go-rtmp dependency) |

### Frontend (CDN)

| Library | URL | Purpose |
|---|---|---|
| HLS.js | `cdn.jsdelivr.net/npm/hls.js@latest` | Adaptive HLS playback in browsers |

### System tools (must be in `$PATH`)

| Tool | Version | Purpose |
|---|---|---|
| `ffmpeg` | 4.0+ recommended | Video encoding, segmentation, live transcoding |
| `ffprobe` | same as ffmpeg | Video file inspection (resolution, codec, duration) |
