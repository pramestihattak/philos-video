# Philos Video — Codebase Context Skill

## Overview
Self-hosted Go video streaming platform (Go 1.22). Supports VOD upload/transcode, JWT-protected HLS delivery, RTMP live ingest, and a real-time QoE dashboard.

**Module:** `philos-video`
**Working dir:** `/Users/pram/Workspace/@opam22/philos-video`
**Primary entry point:** `cmd/server/main.go` (HTTP port 8080)

---

## Architecture

### HTTP Server
- **Router:** Go stdlib `net/http` — routes registered as `"METHOD /path"` patterns (Go 1.22 pattern routing)
- **Middleware stack:** auth (`RequirePlaybackToken`, `RequireLiveToken`), IP rate limiter (`middleware.NewIPRateLimiter`), PIN gate (`GoLivePinGate`)
- **No framework** — handlers are plain `http.HandlerFunc`

### Key Packages
| Package | Purpose |
|---------|---------|
| `cmd/server` | Wires everything together; graceful shutdown |
| `internal/config` | Env-var config; fatal on insecure defaults |
| `internal/database` | `Connect()` (with pool config) + inline migration SQL |
| `internal/handler` | HTTP handlers (upload, video, session, telemetry, dashboard, live, page) |
| `internal/middleware` | Auth JWT, GoLive PIN gate, IP rate limiter |
| `internal/repository` | SQL queries (video, upload, job, session, event, stream_key, live_stream) |
| `internal/service` | Business logic (upload assembly, transcode orchestration, JWT sessions, video delete) |
| `internal/transcoder` | FFmpeg/FFprobe wrappers: probe, encode, segment, manifest |
| `internal/worker` | Goroutine pool that drains the job channel |
| `internal/live` | RTMP ingest → FFmpeg → HLS; `Manager` + `transcodeSession` |
| `internal/qoe` | In-memory 5-min sliding window QoE aggregator; SSE broadcast |
| `internal/web` | Embedded HTML templates (`embed.go`) |

### Data Flow — VOD Upload
```
Client → PUT /api/v1/uploads/{id}/chunks/{n}
       → UploadService.ReceiveChunk() writes chunk file
       → Last chunk triggers assemble() goroutine (2h timeout)
       → Raw file assembled → TranscodeJob queued
       → TranscodeWorker picks job → TranscodeService.Process()
       → FFprobe → BuildLadder → Encode × N → Segment × N → WriteManifest
       → video.status = "ready"
```

### Data Flow — Live Streaming
```
FFmpeg/OBS → RTMP :1935/live/{stream_key}
           → RTMPHandler validates stream key → Manager.StartStream()
           → transcodeSession spawns FFmpeg (stdin pipe, FLV wrapping)
           → FFmpeg writes .ts segments to data/live/{stream_id}/
           → Client fetches HLS with JWT token
           → On disconnect → Manager.EndStream() → convertToVOD()
```

---

## Database (PostgreSQL 15, port 5433)

### Tables
- `videos` — id, title, status (uploading/processing/ready/failed), width, height, duration, codec, hls_path
- `upload_chunks` — upload_id, chunk_number, received
- `transcode_jobs` — id, video_id, status (queued/running/done/failed), stage, progress, error
- `playback_sessions` — id, video_id (nullable), stream_id, token, device_type, user_agent, ip_address, started_at, last_active_at, ended_at, status
- `playback_events` — BIGSERIAL id, session_id, video_id, event_type, timestamp + 12 metric columns
- `stream_keys` — id, user_label, is_active
- `live_streams` — id, stream_key_id, title, status (waiting/live/ended), source info, hls_path, video_id

### Key Indexes
- `idx_playback_events_session`, `idx_playback_events_type_time`, `idx_playback_events_video`
- `idx_live_streams_status`, `idx_live_streams_stream_key`
- `idx_playback_sessions_status_active`, `idx_playback_events_session_time`

### Connection Pool
`SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5m)`

---

## JWT / Auth
- **Library:** `github.com/golang-jwt/jwt/v5`
- **Claims:** `PlaybackClaims{sid, vid, stid}` in `internal/service/session_service.go`
- **Token delivery:** `?token=` query param on HLS requests (Safari limitation — no custom headers on video loads)
- **Config:** `JWT_SECRET` must be set (server refuses to start with the default); `JWT_EXPIRY` defaults to `1h`
- **Live auth:** Same JWT flow, `stid` claim carries `stream_id`

---

## Environment Variables
| Variable | Default | Notes |
|----------|---------|-------|
| `JWT_SECRET` | *(none)* | **Required** — server exits if using code default |
| `PORT` | 8080 | HTTP listen port |
| `RTMP_PORT` | 1935 | RTMP listen port |
| `DATABASE_URL` | postgres://philos:philos@localhost:5433/philos_video?sslmode=disable | |
| `DATA_DIR` | ./data | Root for chunks/raw/hls/live subdirs |
| `WORKER_COUNT` | 2 | Parallel transcode goroutines |
| `JWT_EXPIRY` | 1h | JWT token lifetime (no refresh) |
| `GO_LIVE_PIN` | *(empty)* | If unset, /go-live is public; warning logged at startup |

---

## Data Directory Layout
```
data/
  chunks/{upload_id}/     — raw uploaded chunks (deleted after assembly)
  raw/{upload_id}/        — assembled input file (deleted after transcode)
  hls/{video_id}/         — VOD HLS output (master.m3u8, 720p/, 480p/, 360p/)
  live/{stream_id}/       — live HLS output (.ts segments, sliding window)
```
Directories created with `0o700` (private to server process).

---

## FFmpeg Usage Patterns
- **Probe:** `ffprobe -v quiet -print_format json -show_streams` → `transcoder.Probe()`
- **VOD encode:** `ffmpeg -i input -c:v libx264 ... output.mp4` → then `ffmpeg` re-mux to HLS fMP4 segments
- **Live transcode:** FFmpeg reads from stdin (FLV-wrapped RTMP payload), outputs MPEG-TS 2s segments, 5-segment sliding window
- **FLV wrapping:** `internal/live/transcode_session.go` manually prepends FLV tag headers to RTMP payloads before writing to FFmpeg stdin

---

## Known Limitations
1. **Safari + JWT:** Safari's native HLS player doesn't forward `?token=` on sub-playlist requests; use hls.js in browser
2. **No JWT refresh:** Set `JWT_EXPIRY` longer than your longest expected session
3. **Live audio required:** Video-only RTMP streams fail FFmpeg (audio -map assumed)
4. **Server restart = lost live streams:** After crash, run `UPDATE live_streams SET status='ended' WHERE status='live'`
5. **No user auth:** All VOD APIs are public (stream keys give minimal broadcaster protection)
6. **No job retry:** Failed transcode = must re-upload (retry counter not implemented)

---

## Phase History
- **Phase 2:** Chunked upload API, PostgreSQL, transcode workers, multi-page web UI
- **Phase 3:** JWT sessions, telemetry pipeline (playback_events), QoE dashboard with SSE
- **Phase 4:** RTMP live ingest, multi-quality live HLS, VOD-from-live recording

---

## Common Debugging
- **Upload stuck:** Check `transcode_jobs` table for `status='running'` and `stage`; check server logs for FFmpeg stderr
- **HLS 403:** JWT expired or video_id mismatch in claims — re-create session
- **Live stream not starting:** Check RTMP port 1935 is reachable; verify stream key is active in DB
- **DB connection issues:** `docker compose up -d` then check `DATABASE_URL` env var
- **Build:** `go build ./...` — all packages; `go test ./... -race` for tests with race detector
