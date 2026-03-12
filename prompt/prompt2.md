# Claude Code Prompt: Video Streaming Platform — Phase 2

## Context

I'm building a video streaming platform from scratch. I've already completed Phase 1, which gives me:

- A Go CLI that transcodes video into multiple HLS qualities using FFmpeg (720p/480p/360p)
- A Go HTTP server that serves HLS segments with correct Content-Types
- A web player using hls.js with ABR quality switching

The existing code lives in this project structure:
```
video-streaming/
├── cmd/
│   ├── transcode/main.go
│   └── server/main.go
├── internal/
│   ├── transcoder/
│   │   ├── probe.go
│   │   ├── encode.go
│   │   ├── segment.go
│   │   ├── manifest.go
│   │   └── ladder.go
│   └── server/
│       ├── handler.go
│       └── player.go
├── go.mod
├── Makefile
└── README.md
```

Now I'm building Phase 2: turning this into an actual platform with uploads, background processing, and a video library.

## What To Build

### 1. Unified Server: `go run cmd/server/main.go`

Replace the two separate commands with a single server that handles everything:
- API endpoints (upload, video metadata, job status)
- Serves transcoded video files (HLS segments/manifests)
- Serves the web UI (upload page + video library + player)
- Runs background transcode workers in goroutines

### 2. Database Layer (PostgreSQL)

**Videos table:**
```sql
CREATE TABLE videos (
    id          TEXT PRIMARY KEY,            -- nanoid or uuid
    title       TEXT NOT NULL,
    filename    TEXT NOT NULL,               -- original filename
    file_size   BIGINT NOT NULL,             -- bytes
    duration    FLOAT,                       -- seconds, populated after probe
    resolution  TEXT,                        -- e.g. "1920x1080", populated after probe
    status      TEXT NOT NULL DEFAULT 'uploading',  -- uploading, processing, ready, failed
    error_msg   TEXT,                        -- populated on failure
    raw_path    TEXT,                        -- path to raw uploaded file
    hls_path    TEXT,                        -- path to HLS output directory
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);
```

**Upload chunks tracking table:**
```sql
CREATE TABLE upload_chunks (
    upload_id    TEXT NOT NULL,
    chunk_number INTEGER NOT NULL,
    received     BOOLEAN DEFAULT FALSE,
    received_at  TIMESTAMP,
    PRIMARY KEY (upload_id, chunk_number)
);
```

**Transcode jobs table:**
```sql
CREATE TABLE transcode_jobs (
    id          TEXT PRIMARY KEY,
    video_id    TEXT NOT NULL REFERENCES videos(id),
    status      TEXT NOT NULL DEFAULT 'queued',  -- queued, running, complete, failed
    progress    FLOAT DEFAULT 0,                 -- 0.0 to 1.0
    stage       TEXT,                            -- 'probing', 'encoding_720p', 'segmenting', etc.
    started_at  TIMESTAMP,
    completed_at TIMESTAMP,
    error_msg   TEXT,
    created_at  TIMESTAMP DEFAULT NOW()
);
```

Use `database/sql` with `github.com/lib/pq` driver. Follow repository pattern:
- `internal/repository/video_repo.go`
- `internal/repository/upload_repo.go`
- `internal/repository/job_repo.go`

### 3. Chunked Upload API

**POST /api/v1/uploads** — Initialize upload
```json
// Request
{ "filename": "my-video.mp4", "file_size": 536870912, "title": "My Video" }

// Response
{ "upload_id": "abc123", "chunk_size": 5242880, "total_chunks": 103 }
```
- Chunk size: 5MB fixed
- Calculate total_chunks from file_size
- Create video record (status: "uploading") and upload_chunks records
- Generate upload_id (use nanoid or UUID)

**PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}** — Upload a single chunk
- Accept raw bytes in request body
- Stream directly to disk (do NOT buffer entire chunk in memory): use `io.Copy` to a file
- Store chunks as: `./data/chunks/{upload_id}/{chunk_number:05d}`
- Mark chunk as received in database
- If all chunks received: trigger assembly

**GET /api/v1/uploads/{upload_id}/status** — Check upload progress
```json
// Response
{
  "upload_id": "abc123",
  "total_chunks": 103,
  "received_chunks": 47,
  "status": "uploading"
}
```

**Assembly (triggered when all chunks received):**
1. Concatenate all chunk files into single file: `./data/raw/{video_id}/{filename}`
2. Delete chunk files (cleanup)
3. Update video status to "processing"
4. Create transcode_job record (status: "queued")
5. Signal the background worker (use a Go channel)

### 4. Background Transcode Worker

Run as goroutines inside the same server process (no separate worker binary for now).

```go
type TranscodeWorker struct {
    jobChan    chan string  // receives job IDs
    numWorkers int         // number of concurrent workers (default: 2)
    // ... repos, transcoder service
}
```

**Worker loop:**
1. Receive job_id from channel
2. Update job status to "running", set started_at
3. **Probe** the raw video with FFprobe (reuse existing probe.go)
   - Update video record with duration, resolution
   - Update job stage: "probing", progress: 0.05
4. **Build encoding ladder** (reuse existing ladder.go)
   - Filter out qualities above source resolution
   - Update job stage: "preparing", progress: 0.10
5. **Transcode each quality level** (reuse existing encode.go)
   - For each profile, update job stage: "encoding_{name}" (e.g., "encoding_720p")
   - Parse FFmpeg stderr to extract progress (frame count / total frames)
   - Update job progress proportionally: encoding takes 10%-80% of total progress
   - Output to: `./data/hls/{video_id}/`
6. **Segment each quality** (reuse existing segment.go)
   - Update job stage: "segmenting", progress: 0.85
7. **Generate master playlist** (reuse existing manifest.go)
   - Update job stage: "packaging", progress: 0.95
8. **Finalize:**
   - Update video: status="ready", hls_path="./data/hls/{video_id}"
   - Update job: status="complete", progress=1.0, completed_at=now
   - Delete raw file (optional — could keep for re-encoding later)

**Error handling:**
- If any step fails: update job status="failed", error_msg=err.Error()
- Update video status="failed", error_msg=err.Error()
- Log the full FFmpeg stderr on encode failures
- Do NOT crash the worker — log the error and continue to next job

### 5. Video Library API

**GET /api/v1/videos** — List all videos
```json
{
  "videos": [
    {
      "id": "abc123",
      "title": "My Video",
      "duration": 125.5,
      "resolution": "1920x1080",
      "status": "ready",
      "created_at": "2025-01-15T10:00:00Z"
    }
  ]
}
```

**GET /api/v1/videos/{id}** — Get single video details
```json
{
  "id": "abc123",
  "title": "My Video",
  "duration": 125.5,
  "resolution": "1920x1080",
  "status": "ready",
  "manifest_url": "/videos/abc123/master.m3u8",
  "created_at": "2025-01-15T10:00:00Z"
}
```

**GET /api/v1/videos/{id}/status** — Get processing status (for polling during upload)
```json
{
  "video_status": "processing",
  "job_status": "running",
  "job_progress": 0.45,
  "job_stage": "encoding_720p"
}
```

**Serve HLS files:**
- GET /videos/{video_id}/master.m3u8
- GET /videos/{video_id}/{quality}/playlist.m3u8
- GET /videos/{video_id}/{quality}/init.mp4
- GET /videos/{video_id}/{quality}/segment_*.m4s
- Serve from `./data/hls/{video_id}/` with correct Content-Types

### 6. Web UI (embedded in Go server, served at `/`)

A single-page application (can be simple multi-page HTML too) with three views. Use vanilla HTML/CSS/JavaScript — no React, no build step. Embed as Go embed files or string constants.

**View 1: Video Library (home page at `/`)**
- Grid of video cards showing: thumbnail (or placeholder), title, duration, status badge
- Status badges: "Uploading" (yellow), "Processing" (blue with progress %), "Ready" (green), "Failed" (red)
- Click a "Ready" video → go to player view
- "Upload" button in the top right → go to upload view
- Auto-refresh every 3 seconds to update processing status (simple polling with fetch)

**View 2: Upload (`/upload`)**
- Drag-and-drop zone OR file picker (accept video/* files)
- Title input field
- Upload button
- Progress bar showing:
  - Chunk upload progress (e.g., "Uploading: chunk 47/103 — 45%")
  - After upload completes: "Processing..." with the transcode progress from API polling
- JavaScript implements chunked upload:
  ```
  1. Read file, calculate chunks (5MB each)
  2. POST /api/v1/uploads to initialize
  3. For each chunk: PUT /api/v1/uploads/{id}/chunks/{n} with slice of file
  4. Send chunks sequentially (simpler than parallel for now)
  5. After last chunk: poll GET /api/v1/videos/{id}/status every 2 seconds
  6. When status = "ready": show "Done!" link to player
  ```

**View 3: Player (`/watch/{video_id}`)**
- Reuse the hls.js player from Phase 1
- Load manifest from `/videos/{video_id}/master.m3u8`
- Show video title above the player
- Quality indicator overlay (current quality, bandwidth, buffer level)
- Manual quality selector
- Back button to library

### Project Structure (Updated)

```
video-streaming/
├── cmd/
│   └── server/
│       └── main.go               # Single entry point
├── internal/
│   ├── config/
│   │   └── config.go             # Server config (port, db url, data dir, worker count)
│   ├── database/
│   │   └── postgres.go           # DB connection + migration runner
│   ├── models/
│   │   └── models.go             # Video, UploadChunk, TranscodeJob structs
│   ├── repository/
│   │   ├── video_repo.go
│   │   ├── upload_repo.go
│   │   └── job_repo.go
│   ├── service/
│   │   ├── upload_service.go     # Upload init, chunk handling, assembly
│   │   ├── transcode_service.go  # Wraps the transcoder package with job management
│   │   └── video_service.go      # Video CRUD, status queries
│   ├── handler/
│   │   ├── upload_handler.go     # Upload API endpoints
│   │   ├── video_handler.go      # Video API + HLS file serving
│   │   └── page_handler.go       # Serves HTML pages
│   ├── worker/
│   │   └── transcode_worker.go   # Background worker pool
│   ├── transcoder/                # (existing from Phase 1)
│   │   ├── probe.go
│   │   ├── encode.go
│   │   ├── segment.go
│   │   ├── manifest.go
│   │   └── ladder.go
│   └── web/
│       ├── templates/
│       │   ├── library.html      # Video grid
│       │   ├── upload.html       # Upload page
│       │   └── player.html       # Video player
│       └── embed.go              # go:embed for templates
├── migrations/
│   └── 001_initial.sql           # CREATE TABLE statements
├── data/                          # Created at runtime (gitignored)
│   ├── chunks/                    # Temporary chunk storage
│   ├── raw/                       # Assembled raw uploads
│   └── hls/                       # Transcoded HLS output
├── go.mod
├── go.sum
├── Makefile
├── docker-compose.yml             # PostgreSQL for local dev
└── README.md
```

## Technical Requirements

- **Go 1.21+**, use `net/http` with `http.ServeMux` (Go 1.22 enhanced routing) — no frameworks
- **PostgreSQL 15+** — provide a `docker-compose.yml` for easy local setup
- **`database/sql`** with **`github.com/lib/pq`** — no ORM
- **`log/slog`** for structured logging everywhere with context fields (video_id, job_id, upload_id)
- **`go:embed`** for embedding HTML templates
- **`html/template`** for rendering pages
- Run DB migrations on server startup (read .sql files and execute)
- Configuration via environment variables:
  ```
  PORT=8080
  DATABASE_URL=postgres://user:pass@localhost:5432/videostream?sslmode=disable
  DATA_DIR=./data
  WORKER_COUNT=2
  ```

## Key Implementation Details

### Chunked upload streaming (handler)
```go
// Stream chunk directly to disk — do NOT use ioutil.ReadAll or buffer in memory
func (h *UploadHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
    // ...
    chunkPath := filepath.Join(h.dataDir, "chunks", uploadID, fmt.Sprintf("%05d", chunkNum))
    os.MkdirAll(filepath.Dir(chunkPath), 0755)

    f, _ := os.Create(chunkPath)
    defer f.Close()
    io.Copy(f, r.Body)  // streams directly, constant memory
    // ...
}
```

### Assembly (service)
```go
func (s *UploadService) Assemble(ctx context.Context, uploadID string) error {
    // 1. Open output file
    // 2. Iterate chunks in order: 00000, 00001, 00002, ...
    // 3. io.Copy each chunk file into output file (streaming, constant memory)
    // 4. Delete chunk directory
    // 5. Create transcode job, send to worker channel
}
```

### Worker channel pattern
```go
// In main.go
jobChan := make(chan string, 100)  // buffered channel for job IDs

// Start workers
for i := 0; i < cfg.WorkerCount; i++ {
    go worker.Run(ctx, jobChan)
}

// When upload completes, send job to channel
jobChan <- jobID
```

### On server startup
1. Connect to PostgreSQL
2. Run migrations
3. Create data directories (chunks, raw, hls)
4. Start background workers
5. Register routes
6. Start HTTP server
7. Also: scan for any "queued" jobs in DB (from previous crash) and re-enqueue them

### docker-compose.yml
```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: videostream
      POSTGRES_PASSWORD: videostream
      POSTGRES_DB: videostream
    ports:
      - "5433:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
```

### Makefile targets
```makefile
.PHONY: db serve dev clean

db:                      ## Start PostgreSQL
	docker compose up -d

serve:                   ## Run the server
	go run cmd/server/main.go

dev: db serve            ## Start everything

clean:                   ## Remove data directory
	rm -rf ./data

stop:                    ## Stop PostgreSQL
	docker compose down
```

## What This Phase Teaches Me

- How chunked, resumable uploads work (streaming to disk, never buffering)
- How background job processing works (channel-based worker pool in Go)
- How to connect the upload → process → serve pipeline end-to-end
- How to track async job progress and surface it in the UI
- The 3-layer architecture (handler → service → repository) applied to a video platform
- How to serve both API and static files from the same Go server

## Important Notes

- Reuse as much of the Phase 1 transcoder code as possible — the core FFmpeg logic shouldn't change
- The main new code is: upload handling, database layer, worker pool, and web UI
- Keep it simple: no authentication, no authorization, single-user for now
- No file validation beyond checking the extension — we'll add proper validation later
- If PostgreSQL connection fails on startup, print a helpful message suggesting `make db`
- The web UI should be functional but doesn't need to be pretty — clean and usable is enough