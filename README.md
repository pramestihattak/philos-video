# philos-video

A self-hosted video streaming platform written in Go. Supports chunked upload, server-side transcoding to adaptive-bitrate HLS, live RTMP ingest with real-time multi-quality HLS delivery, JWT-secured playback, and a live QoE dashboard.

> **Full documentation:** [`docs/README.md`](docs/README.md)

---

## Features

- **Chunked upload** — browser-side file splitting, sequential 5 MB chunks, auto-assembles on last chunk
- **VOD transcoding** — FFmpeg pipeline: probe → encode (720p/480p/360p) → fMP4 HLS segments → master playlist
- **Live streaming** — RTMP ingest (port 1935), real-time multi-quality HLS with 2-second segments
- **JWT playback auth** — signed tokens bound to specific video or stream IDs
- **Telemetry pipeline** — client-side event collection (TTFF, rebuffer, quality switches, throughput)
- **QoE dashboard** — live metrics via SSE: rebuffer rate, TTFF percentiles, bitrate distribution, throughput

---

## Quick Start

```bash
# Requirements: Go 1.22+, FFmpeg, Docker

make db       # start PostgreSQL in Docker
make serve    # start HTTP (:8080) + RTMP (:1935) server

open http://localhost:8080          # video library
open http://localhost:8080/upload   # upload a video
open http://localhost:8080/go-live  # manage stream keys (OBS live streaming)
open http://localhost:8080/dashboard
```

Migrations run automatically on startup — no manual SQL needed.

---

## Architecture

```
Browser ──────┐
              │   Go HTTP Server (:8080)
OBS/Encoder ──┤   ├── Upload API    /api/v1/uploads/…
  RTMP :1935  │   ├── Video API     /api/v1/videos/…
              │   ├── Session API   /api/v1/…/sessions
              │   ├── Telemetry     /api/v1/sessions/…/events
              │   ├── Dashboard SSE /api/v1/dashboard/stats/stream
              │   └── HLS serving   /videos/…  /live/…  (JWT-protected)
              └──────────────────────────┬──────────────────────────
                                         │
                              PostgreSQL 15 (:5433)
```

### Component layers

| Layer | Package | Purpose |
|-------|---------|---------|
| Entry point | `cmd/server` | Wire config → DB → services → routes |
| Handlers | `internal/handler` | HTTP request/response |
| Services | `internal/service` | Business logic |
| Repositories | `internal/repository` | SQL queries |
| Live streaming | `internal/live` | RTMP server + per-stream FFmpeg sessions |
| VOD transcoding | `internal/transcoder` | FFmpeg/FFprobe wrappers |
| QoE | `internal/qoe` | In-memory 5-min sliding window metrics |

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://philos:philos@localhost:5433/philos_video?sslmode=disable` | PostgreSQL DSN |
| `DATA_DIR` | `./data` | Video storage root |
| `WORKER_COUNT` | `2` | Parallel transcode workers |
| `JWT_SECRET` | dev default | **Change in production** (min 32 chars) |
| `JWT_EXPIRY` | `1h` | Playback token lifetime |
| `RTMP_PORT` | `1935` | RTMP ingest port |

---

## Makefile

| Target | Description |
|--------|-------------|
| `make db` | Start PostgreSQL via Docker Compose |
| `make stop` | Stop PostgreSQL |
| `make serve` | Run HTTP + RTMP server |
| `make dev` | Live-reload dev server (uses `air` if available) |
| `make build` | Compile binaries to `bin/` |
| `make transcode INPUT=… OUTPUT=…` | CLI batch transcode (no server needed) |
| `make clean` | Remove `bin/` and `data/` |

---

## Live Streaming (OBS)

1. Go to `http://localhost:8080/go-live` and create a stream key (`sk_…`)
2. In OBS: **Settings → Stream → Service: Custom**
   - Server: `rtmp://localhost:1935/live`
   - Stream Key: the `sk_…` value
3. Start streaming — viewers watch at `/watch-live/{stream_id}`
4. On stream end, a VOD recording is automatically created in the library

---

## Encoding Ladder

| Profile | Resolution | Video | Audio | Format |
|---------|-----------|-------|-------|--------|
| 720p | 1280×720 | 2500 kbps | 128 kbps | HLS fMP4 (VOD) / TS (live) |
| 480p | 854×480 | 1000 kbps | 96 kbps | HLS fMP4 (VOD) / TS (live) |
| 360p | 640×360 | 400 kbps | 64 kbps | HLS fMP4 (VOD) / TS (live) |

Profiles exceeding source resolution are skipped automatically.

---

## Documentation

See [`docs/README.md`](docs/README.md) for comprehensive documentation covering:

- Complete database schema (all 6 tables with every column)
- Full HTTP API reference (all routes, request/response shapes)
- Authentication & JWT token structure
- VOD and live pipeline internals
- QoE aggregator design and metric calculations
- Step-by-step guide for adding features
- Deployment (systemd, nginx reverse proxy)
- Security considerations and known limitations
