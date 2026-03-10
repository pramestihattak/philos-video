# Claude Code Prompt: Video Streaming Platform — Phase 4

## Context

I'm building a video streaming platform from scratch. I've completed:

- **Phase 1:** FFmpeg transcoding into multi-quality HLS, hls.js player with ABR
- **Phase 2:** Chunked upload API, PostgreSQL, background transcode workers, web UI (library, upload, player)
- **Phase 3:** Playback sessions with signed JWT URLs, client telemetry pipeline (TTFF, rebuffer, bandwidth, quality switches), QoE aggregator with real-time SSE dashboard

The platform currently handles VOD (Video on Demand) end-to-end. Now I'm adding live streaming.

## What To Build

### Overview

Four new capabilities:

1. **RTMP Ingest Server** — Accept incoming RTMP streams from OBS/Streamlabs, authenticate via stream key, relay to transcoder
2. **Real-Time Transcoder** — FFmpeg running continuously, ingesting the RTMP feed, outputting live HLS segments in multiple qualities
3. **Live HLS Serving** — Sliding window manifests that update every segment, served through the existing signed URL infrastructure
4. **Live UI** — "Go Live" page to get a stream key, live stream viewer page, live indicator in library, and live stats in QoE dashboard

After this, a user can: get a stream key → paste it in OBS → go live → viewers watch in browser with ABR quality switching → see live metrics in dashboard → stream ends → recording automatically becomes a VOD.

```
OBS (RTMP) → Ingest Server → FFmpeg (real-time) → Live HLS segments
                                                        ↓
                                    Viewers ← hls.js ← /live/{stream_id}/master.m3u8
                                                        ↓
                                    On stream end → VOD recording available
```

---

### 1. Database Changes

#### New Tables

```sql
CREATE TABLE stream_keys (
    id          TEXT PRIMARY KEY,              -- the stream key itself (used in RTMP URL)
    user_label  TEXT NOT NULL,                 -- human label like "My Stream Key"
    is_active   BOOLEAN DEFAULT TRUE,          -- can be disabled
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE live_streams (
    id              TEXT PRIMARY KEY,           -- stream session ID
    stream_key_id   TEXT NOT NULL REFERENCES stream_keys(id),
    title           TEXT NOT NULL DEFAULT 'Untitled Stream',
    status          TEXT NOT NULL DEFAULT 'connecting', 
                    -- connecting, live, ending, ended, failed
    
    -- Source properties (detected from incoming RTMP stream)
    source_codec    TEXT,                      -- 'h264'
    source_resolution TEXT,                    -- '1920x1080'
    source_framerate FLOAT,                   -- 30.0
    source_bitrate  BIGINT,                   -- bps
    
    -- Infrastructure
    ingest_server   TEXT,                      -- hostname handling this stream
    ffmpeg_pid      INTEGER,                   -- PID of the live FFmpeg process
    hls_path        TEXT,                      -- path to live HLS output directory
    
    -- Timing
    started_at      TIMESTAMP,                 -- when first frame received
    ended_at        TIMESTAMP,                 -- when stream ended
    duration        FLOAT,                     -- seconds, computed on end
    
    -- VOD recording
    video_id        TEXT REFERENCES videos(id), -- linked VOD after stream ends
    
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- Index for finding currently live streams
CREATE INDEX idx_live_streams_status ON live_streams(status) WHERE status = 'live';
CREATE INDEX idx_live_streams_key ON live_streams(stream_key_id);
```

#### Migration: `003_live_streaming.sql`

---

### 2. RTMP Ingest Server

Use the Go library `github.com/yutopp/go-rtmp` to accept RTMP connections. This library provides a server that handles the RTMP handshake and exposes the audio/video data.

**However**, for simplicity and reliability, I recommend a different approach: **use FFmpeg itself as the relay**. Here's the architecture:

#### Approach: Lightweight RTMP relay using `github.com/yutopp/go-rtmp`

The RTMP server accepts the connection, authenticates the stream key, then pipes the raw RTMP data to an FFmpeg process via stdin (or more practically, has FFmpeg pull from a local RTMP relay).

**Simpler approach that I want to use:**

1. Run a minimal RTMP server in Go using `github.com/yutopp/go-rtmp`
2. On new stream: authenticate stream key, create live_stream record
3. The RTMP server writes the incoming FLV data to a pipe
4. FFmpeg reads from that pipe and produces live HLS output

**Actually, the simplest reliable approach:**

1. Run a minimal RTMP server using `github.com/yutopp/go-rtmp`
2. On connect: validate stream key from the RTMP URL path
3. On publish: start FFmpeg process that pulls from a local relay
4. The Go RTMP handler relays incoming packets to a local TCP port that FFmpeg reads from

**Let me simplify even further. The most practical approach:**

Use `github.com/yutopp/go-rtmp` as the RTMP server. When a stream starts publishing, write the incoming H.264/AAC data to a named pipe (FIFO). FFmpeg reads from the FIFO and outputs HLS.

```
OBS → RTMP → Go RTMP Server (auth + relay) → Named Pipe (FIFO)
                                                    ↓
                                              FFmpeg reads FIFO
                                                    ↓
                                              Live HLS output
```

#### Implementation

```go
// internal/live/rtmp_server.go

type RTMPServer struct {
    port          int
    streamKeyRepo repository.StreamKeyRepo
    liveService   *LiveService
    logger        *slog.Logger
}

func (s *RTMPServer) Start(ctx context.Context) error {
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
    if err != nil {
        return fmt.Errorf("rtmp listen failed: %w", err)
    }
    
    s.logger.Info("rtmp_server_started", slog.Int("port", s.port))
    
    srv := rtmp.NewServer(&rtmp.ServerConfig{
        OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
            return conn, &rtmp.ConnConfig{
                Handler: &rtmpHandler{
                    server: s,
                    conn:   conn,
                    logger: s.logger,
                },
            }
        },
    })
    
    go func() {
        <-ctx.Done()
        listener.Close()
    }()
    
    return srv.Serve(listener)
}
```

#### RTMP Handler

```go
// internal/live/rtmp_handler.go

type rtmpHandler struct {
    server    *RTMPServer
    conn      net.Conn
    logger    *slog.Logger
    streamKey string
    streamID  string
    session   *LiveTranscodeSession
}

// Called when OBS connects
func (h *rtmpHandler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
    h.logger.Info("rtmp_connect", 
        slog.String("remote_addr", h.conn.RemoteAddr().String()),
        slog.String("app", cmd.Command.App),  // e.g., "live"
    )
    return nil
}

// Called when OBS starts publishing
func (h *rtmpHandler) OnPublish(timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
    // The stream key is typically passed as the stream name
    // OBS URL: rtmp://localhost:1935/live
    // OBS Stream Key: sk_abc123def456
    // This results in cmd.PublishingName = "sk_abc123def456"
    
    h.streamKey = cmd.PublishingName
    
    h.logger.Info("rtmp_publish_request",
        slog.String("stream_key", h.streamKey),
        slog.String("remote_addr", h.conn.RemoteAddr().String()),
    )
    
    // 1. Validate stream key
    key, err := h.server.streamKeyRepo.GetByID(context.Background(), h.streamKey)
    if err != nil || !key.IsActive {
        h.logger.Warn("rtmp_auth_failed", slog.String("stream_key", h.streamKey))
        return fmt.Errorf("invalid or inactive stream key")
    }
    
    // 2. Check for duplicate streams (same key already live)
    existing, _ := h.server.liveService.GetActiveByStreamKey(context.Background(), h.streamKey)
    if existing != nil {
        h.logger.Warn("rtmp_duplicate_stream", slog.String("stream_key", h.streamKey))
        // End existing stream before starting new one
        h.server.liveService.EndStream(context.Background(), existing.ID, "replaced")
    }
    
    // 3. Create live stream record
    stream, err := h.server.liveService.CreateStream(context.Background(), h.streamKey)
    if err != nil {
        return fmt.Errorf("failed to create stream: %w", err)
    }
    h.streamID = stream.ID
    
    // 4. Start the live transcode session
    h.session, err = h.server.liveService.StartTranscode(context.Background(), stream)
    if err != nil {
        return fmt.Errorf("failed to start transcode: %w", err)
    }
    
    h.logger.Info("rtmp_stream_started",
        slog.String("stream_id", h.streamID),
        slog.String("stream_key", h.streamKey),
    )
    
    return nil
}

// Called for each audio packet
func (h *rtmpHandler) OnAudio(timestamp uint32, payload io.Reader) error {
    if h.session == nil {
        return nil
    }
    return h.session.WriteAudio(timestamp, payload)
}

// Called for each video packet
func (h *rtmpHandler) OnVideo(timestamp uint32, payload io.Reader) error {
    if h.session == nil {
        return nil
    }
    return h.session.WriteVideo(timestamp, payload)
}

// Called when OBS disconnects
func (h *rtmpHandler) OnClose() {
    h.logger.Info("rtmp_disconnect",
        slog.String("stream_id", h.streamID),
        slog.String("stream_key", h.streamKey),
    )
    
    if h.streamID != "" {
        // End the stream gracefully
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        h.server.liveService.EndStream(ctx, h.streamID, "disconnected")
    }
}
```

**Note on the RTMP library:** `github.com/yutopp/go-rtmp` may have a slightly different API than shown above. When implementing, check the library's actual interface and adapt accordingly. The key concepts are:
- Accept TCP connections on port 1935
- Extract the stream key from the publish command
- Validate it against the database
- Forward audio/video data to FFmpeg

If `go-rtmp` proves too complex, an alternative approach is:
1. Use a simple TCP listener
2. Have FFmpeg listen on a local RTMP port: `ffmpeg -listen 1 -i rtmp://localhost:1936/...`
3. Your Go server acts as a TCP proxy: accepts on port 1935, authenticates, then pipes bytes to FFmpeg's port 1936

Choose whichever approach works. The important thing is: RTMP in → authenticate → FFmpeg gets the data.

---

### 3. Live Transcode Session

This manages the FFmpeg process that converts the incoming RTMP stream to live HLS.

```go
// internal/live/transcode_session.go

type LiveTranscodeSession struct {
    streamID    string
    stream      *models.LiveStream
    ffmpegCmd   *exec.Cmd
    stdinPipe   io.WriteCloser    // pipe RTMP data to FFmpeg
    outputDir   string            // where HLS segments go
    cancelFunc  context.CancelFunc
    logger      *slog.Logger
    
    // For relaying RTMP data to FFmpeg
    inputPipe   *os.File          // write end of named pipe
}

func (s *LiveService) StartTranscode(ctx context.Context, stream *models.LiveStream) (*LiveTranscodeSession, error) {
    outputDir := filepath.Join(s.dataDir, "live", stream.ID)
    os.MkdirAll(outputDir, 0755)
    
    // Create subdirectories for each quality
    for _, q := range []string{"720p", "480p", "360p"} {
        os.MkdirAll(filepath.Join(outputDir, q), 0755)
    }
    
    // Create named pipe (FIFO) for FFmpeg input
    fifoPath := filepath.Join(outputDir, "input.flv")
    syscall.Mkfifo(fifoPath, 0644)
    
    ctx, cancel := context.WithCancel(ctx)
    
    session := &LiveTranscodeSession{
        streamID:   stream.ID,
        stream:     stream,
        outputDir:  outputDir,
        cancelFunc: cancel,
        logger:     slog.With(slog.String("stream_id", stream.ID)),
    }
    
    // Build and start FFmpeg command
    args := buildLiveFFmpegArgs(fifoPath, outputDir)
    session.ffmpegCmd = exec.CommandContext(ctx, "ffmpeg", args...)
    
    // Capture FFmpeg stderr for logging
    stderrPipe, _ := session.ffmpegCmd.StderrPipe()
    go session.monitorFFmpegOutput(stderrPipe)
    
    // Start FFmpeg (it will block waiting for data on the FIFO)
    if err := session.ffmpegCmd.Start(); err != nil {
        cancel()
        return nil, fmt.Errorf("ffmpeg start failed: %w", err)
    }
    
    // Open the write end of the FIFO (this unblocks FFmpeg)
    inputPipe, err := os.OpenFile(fifoPath, os.O_WRONLY, 0)
    if err != nil {
        cancel()
        return nil, fmt.Errorf("open fifo failed: %w", err)
    }
    session.inputPipe = inputPipe
    
    // Update stream record
    stream.Status = "live"
    stream.FFmpegPID = session.ffmpegCmd.Process.Pid
    stream.HLSPath = outputDir
    stream.StartedAt = timePtr(time.Now())
    s.streamRepo.Update(ctx, stream)
    
    // Monitor FFmpeg process
    go session.monitorProcess(ctx)
    
    session.logger.Info("live_transcode_started",
        slog.Int("ffmpeg_pid", session.ffmpegCmd.Process.Pid),
        slog.String("output_dir", outputDir),
    )
    
    return session, nil
}

func buildLiveFFmpegArgs(inputPath, outputDir string) []string {
    // FFmpeg command for live multi-quality HLS output
    //
    // Key differences from VOD encoding:
    // - `-preset veryfast` instead of `medium` (must be faster than real-time)
    // - `-tune zerolatency` (minimize encoding latency)
    // - `-g 60` (2-second GOP for 30fps, matching 2-second segments)
    // - `-hls_time 2` (2-second segments for lower latency)
    // - `-hls_list_size 5` (sliding window of 5 segments in playlist)
    // - `-hls_flags delete_segments` (remove old segment files)
    // - `-hls_flags +independent_segments` (each segment starts with keyframe)
    // - No `-movflags +faststart` (not applicable for live)
    
    return []string{
        // Input: read from named pipe as FLV
        "-f", "flv",
        "-i", inputPath,
        
        // Split into multiple resolutions
        "-filter_complex",
        "[0:v]split=3[v720][v480][v360];" +
            "[v720]scale=1280:720[v720out];" +
            "[v480]scale=854:480[v480out];" +
            "[v360]scale=640:360[v360out]",
        
        // ── 720p output ──
        "-map", "[v720out]", "-map", "0:a",
        "-c:v:0", "libx264",
        "-preset", "veryfast",
        "-tune", "zerolatency",
        "-b:v:0", "2500k",
        "-maxrate:v:0", "2750k",
        "-bufsize:v:0", "5000k",
        "-g", "60",              // GOP = 2s at 30fps
        "-keyint_min", "60",
        "-sc_threshold", "0",
        "-c:a:0", "aac",
        "-b:a:0", "128k",
        "-ar", "48000",
        "-f", "hls",
        "-hls_time", "2",
        "-hls_list_size", "5",
        "-hls_flags", "delete_segments+independent_segments+append_list",
        "-hls_segment_type", "fmp4",
        "-hls_fmp4_init_filename", "init.mp4",
        "-hls_segment_filename", filepath.Join(outputDir, "720p", "segment_%05d.m4s"),
        filepath.Join(outputDir, "720p", "playlist.m3u8"),
        
        // ── 480p output ──
        "-map", "[v480out]", "-map", "0:a",
        "-c:v:1", "libx264",
        "-preset", "veryfast",
        "-tune", "zerolatency",
        "-b:v:1", "1000k",
        "-maxrate:v:1", "1100k",
        "-bufsize:v:1", "2000k",
        "-g", "60",
        "-keyint_min", "60",
        "-sc_threshold", "0",
        "-c:a:1", "aac",
        "-b:a:1", "128k",
        "-ar", "48000",
        "-f", "hls",
        "-hls_time", "2",
        "-hls_list_size", "5",
        "-hls_flags", "delete_segments+independent_segments+append_list",
        "-hls_segment_type", "fmp4",
        "-hls_fmp4_init_filename", "init.mp4",
        "-hls_segment_filename", filepath.Join(outputDir, "480p", "segment_%05d.m4s"),
        filepath.Join(outputDir, "480p", "playlist.m3u8"),
        
        // ── 360p output ──
        "-map", "[v360out]", "-map", "0:a",
        "-c:v:2", "libx264",
        "-preset", "veryfast",
        "-tune", "zerolatency",
        "-b:v:2", "600k",
        "-maxrate:v:2", "660k",
        "-bufsize:v:2", "1200k",
        "-g", "60",
        "-keyint_min", "60",
        "-sc_threshold", "0",
        "-c:a:2", "aac",
        "-b:a:2", "96k",
        "-ar", "48000",
        "-f", "hls",
        "-hls_time", "2",
        "-hls_list_size", "5",
        "-hls_flags", "delete_segments+independent_segments+append_list",
        "-hls_segment_type", "fmp4",
        "-hls_fmp4_init_filename", "init.mp4",
        "-hls_segment_filename", filepath.Join(outputDir, "360p", "segment_%05d.m4s"),
        filepath.Join(outputDir, "360p", "playlist.m3u8"),
    }
}
```

**IMPORTANT NOTE about FFmpeg multi-output:** The above uses multiple `-f hls` outputs in a single FFmpeg command. FFmpeg supports this but the syntax can be tricky. An alternative approach if this doesn't work:

**Alternative: Separate FFmpeg processes per quality**
```go
// If single-process multi-output is problematic, use separate processes:
// 1. One FFmpeg reads FIFO and outputs raw decoded frames to a shared pipe
// 2. Three FFmpeg processes each read from the pipe and encode one quality
// 
// OR even simpler:
// 1. FFmpeg reads FIFO and outputs to a local RTMP relay on localhost
// 2. Three separate FFmpeg processes each pull from the local relay
//
// For this project, try single-process first. If it fails, fall back to
// having FFmpeg read from the RTMP server directly:

func buildSimpleLiveFFmpegArgs(rtmpURL, outputDir, quality string, width, height int, videoBitrate, audioBitrate string) []string {
    return []string{
        "-i", rtmpURL,  // e.g., "rtmp://localhost:1935/live/sk_abc123"
        "-c:v", "libx264",
        "-preset", "veryfast",
        "-tune", "zerolatency",
        "-vf", fmt.Sprintf("scale=%d:%d", width, height),
        "-b:v", videoBitrate,
        "-g", "60",
        "-keyint_min", "60",
        "-sc_threshold", "0",
        "-c:a", "aac",
        "-b:a", audioBitrate,
        "-ar", "48000",
        "-f", "hls",
        "-hls_time", "2",
        "-hls_list_size", "5",
        "-hls_flags", "delete_segments+independent_segments+append_list",
        "-hls_segment_type", "fmp4",
        "-hls_fmp4_init_filename", "init.mp4",
        "-hls_segment_filename", filepath.Join(outputDir, quality, "segment_%05d.m4s"),
        filepath.Join(outputDir, quality, "playlist.m3u8"),
    }
}
```

#### Live Master Playlist Generation

Unlike VOD where the master playlist is generated once, the live master playlist is static but must exist before viewers connect. Generate it when the stream starts:

```go
func generateLiveMasterPlaylist(outputDir string, profiles []LiveProfile) error {
    var buf strings.Builder
    buf.WriteString("#EXTM3U\n")
    buf.WriteString("#EXT-X-VERSION:7\n\n")
    
    for _, p := range profiles {
        fmt.Fprintf(&buf,
            "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"avc1.64001f,mp4a.40.2\"\n%s/playlist.m3u8\n\n",
            p.TotalBitrate,
            p.Width, p.Height,
            p.Name,
        )
    }
    
    return os.WriteFile(filepath.Join(outputDir, "master.m3u8"), []byte(buf.String()), 0644)
}
```

#### Process Monitoring

```go
func (s *LiveTranscodeSession) monitorProcess(ctx context.Context) {
    err := s.ffmpegCmd.Wait()
    
    if ctx.Err() != nil {
        // Context was canceled — intentional shutdown
        s.logger.Info("live_transcode_stopped", slog.String("reason", "context_canceled"))
    } else if err != nil {
        // FFmpeg crashed
        s.logger.Error("live_transcode_crashed",
            slog.String("error", err.Error()),
        )
    } else {
        // FFmpeg exited cleanly (input ended)
        s.logger.Info("live_transcode_ended")
    }
}

func (s *LiveTranscodeSession) monitorFFmpegOutput(stderr io.Reader) {
    scanner := bufio.NewScanner(stderr)
    for scanner.Scan() {
        line := scanner.Text()
        // Log FFmpeg output at debug level (very verbose)
        // But log errors/warnings at warn level
        if strings.Contains(line, "error") || strings.Contains(line, "Error") {
            s.logger.Warn("ffmpeg_stderr", slog.String("line", line))
        }
    }
}
```

#### Stopping a Stream

```go
func (s *LiveService) EndStream(ctx context.Context, streamID string, reason string) error {
    stream, err := s.streamRepo.Get(ctx, streamID)
    if err != nil {
        return err
    }
    
    s.logger.Info("ending_stream",
        slog.String("stream_id", streamID),
        slog.String("reason", reason),
    )
    
    // 1. Stop FFmpeg process
    session, exists := s.activeSessions[streamID]
    if exists {
        session.cancelFunc()  // cancels context → kills FFmpeg
        session.inputPipe.Close()
        delete(s.activeSessions, streamID)
    }
    
    // 2. Update stream record
    stream.Status = "ended"
    stream.EndedAt = timePtr(time.Now())
    if stream.StartedAt != nil {
        stream.Duration = time.Since(*stream.StartedAt).Seconds()
    }
    s.streamRepo.Update(ctx, stream)
    
    // 3. Convert live HLS to VOD recording (async)
    go s.convertToVOD(context.Background(), stream)
    
    return nil
}

func (s *LiveService) convertToVOD(ctx context.Context, stream *models.LiveStream) {
    s.logger.Info("converting_to_vod", slog.String("stream_id", stream.ID))
    
    // 1. Finalize all variant playlists by adding #EXT-X-ENDLIST
    for _, quality := range []string{"720p", "480p", "360p"} {
        playlistPath := filepath.Join(stream.HLSPath, quality, "playlist.m3u8")
        f, err := os.OpenFile(playlistPath, os.O_APPEND|os.O_WRONLY, 0644)
        if err != nil {
            continue
        }
        f.WriteString("\n#EXT-X-ENDLIST\n")
        f.Close()
    }
    
    // 2. Create a Video record from the stream
    video := &models.Video{
        ID:         generateID(),
        Title:      stream.Title + " (Recording)",
        Filename:   stream.ID + ".m3u8",
        Status:     "ready",
        Duration:   stream.Duration,
        Resolution: stream.SourceResolution,
        HLSPath:    stream.HLSPath,  // reuse the same HLS output
        CreatedAt:  time.Now(),
    }
    s.videoRepo.Create(ctx, video)
    
    // 3. Link the VOD to the stream
    stream.VideoID = video.ID
    s.streamRepo.Update(ctx, stream)
    
    s.logger.Info("vod_created_from_stream",
        slog.String("stream_id", stream.ID),
        slog.String("video_id", video.ID),
    )
    
    // NOTE: In production, you'd queue a background re-transcode job here
    // to re-encode with -preset slow for better compression.
    // For this project, the live HLS segments serve as the VOD directly.
}
```

---

### 4. Live Streaming API

**POST /api/v1/stream-keys** — Generate a new stream key
```json
// Request
{ "label": "My Gaming Stream" }

// Response
{
  "stream_key": "sk_a1b2c3d4e5f6",
  "rtmp_url": "rtmp://localhost:1935/live",
  "label": "My Gaming Stream",
  "full_url": "rtmp://localhost:1935/live/sk_a1b2c3d4e5f6"
}
```

Generate the stream key with a prefix `sk_` followed by a random string (16 chars alphanumeric).

**GET /api/v1/stream-keys** — List your stream keys
```json
{
  "stream_keys": [
    { "id": "sk_a1b2c3d4e5f6", "label": "My Gaming Stream", "is_active": true, "created_at": "..." }
  ]
}
```

**DELETE /api/v1/stream-keys/{id}** — Deactivate a stream key

**GET /api/v1/live** — List currently live streams
```json
{
  "streams": [
    {
      "id": "ls_xyz789",
      "title": "Friday Night Gaming",
      "status": "live",
      "source_resolution": "1920x1080",
      "viewer_count": 3,
      "started_at": "2025-01-15T20:00:00Z",
      "duration_seconds": 1234,
      "manifest_url": "/live/ls_xyz789/master.m3u8"
    }
  ]
}
```

**GET /api/v1/live/{stream_id}** — Get live stream details

**POST /api/v1/live/{stream_id}/sessions** — Create playback session for live stream
(Same as VOD sessions but with stream_id instead of video_id)
```json
// Response
{
  "session_id": "sess_abc123",
  "manifest_url": "/live/ls_xyz789/master.m3u8?token=eyJhbG...",
  "token": "eyJhbGciOiJIUzI1NiJ9...",
  "token_expires_at": "2025-01-15T21:00:00Z",
  "telemetry_url": "/api/v1/sessions/sess_abc123/events",
  "is_live": true
}
```

**PUT /api/v1/live/{stream_id}** — Update stream metadata (title)
```json
// Request
{ "title": "Friday Night Gaming - Episode 42" }
```

**POST /api/v1/live/{stream_id}/end** — Manually end stream (from web UI)

### Live HLS File Serving

Serve live HLS files under `/live/{stream_id}/`:
```
GET /live/{stream_id}/master.m3u8?token=xxx
GET /live/{stream_id}/720p/playlist.m3u8?token=xxx
GET /live/{stream_id}/720p/init.mp4?token=xxx
GET /live/{stream_id}/720p/segment_00042.m4s?token=xxx
```

Same JWT middleware as VOD, but token claims include `stream_id` instead of (or in addition to) `video_id`.

Update the JWT claims:
```go
type PlaybackClaims struct {
    jwt.RegisteredClaims
    SessionID string `json:"sid"`
    VideoID   string `json:"vid,omitempty"`    // for VOD
    StreamID  string `json:"stid,omitempty"`   // for live
}
```

The auth middleware checks the path prefix:
- `/videos/{id}/**` → validate `claims.VideoID`
- `/live/{id}/**` → validate `claims.StreamID`

**Critical: Manifest caching headers for live**
```go
func (h *LiveHandler) ServeLiveFile(w http.ResponseWriter, r *http.Request) {
    // ... resolve file path ...
    
    if strings.HasSuffix(r.URL.Path, ".m3u8") {
        // Playlists change every segment — very short cache
        w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
        w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
    } else if strings.HasSuffix(r.URL.Path, ".m4s") || strings.HasSuffix(r.URL.Path, ".mp4") {
        // Segments are immutable once written — cache aggressively
        w.Header().Set("Cache-Control", "public, max-age=3600")
        if strings.HasSuffix(r.URL.Path, ".m4s") {
            w.Header().Set("Content-Type", "video/iso.bmff")
        } else {
            w.Header().Set("Content-Type", "video/mp4")
        }
    }
    
    http.ServeFile(w, r, filePath)
}
```

---

### 5. Web UI Updates

#### Go Live Page (`/go-live`)

A page where the user can:
1. See their stream keys (create new ones)
2. Set a stream title
3. See the RTMP URL + stream key to paste into OBS
4. See a live preview of their own stream once it starts
5. See viewer count and stream duration
6. "End Stream" button

```html
<!-- Layout sketch -->
<div class="go-live-page">
  <h1>Go Live</h1>
  
  <!-- Stream Key Section -->
  <div class="stream-key-section">
    <h2>Stream Key</h2>
    <div class="key-display">
      <label>RTMP URL:</label>
      <code>rtmp://localhost:1935/live</code>
      
      <label>Stream Key:</label>
      <code class="secret">sk_a1b2c3d4e5f6</code>
      <button onclick="toggleKeyVisibility()">Show/Hide</button>
      <button onclick="copyToClipboard()">Copy</button>
    </div>
    <p class="hint">Paste these into OBS → Settings → Stream</p>
  </div>
  
  <!-- Stream Settings -->
  <div class="stream-settings">
    <label>Title:</label>
    <input type="text" id="stream-title" value="My Stream" />
    <button onclick="updateTitle()">Update</button>
  </div>
  
  <!-- Live Status (hidden until stream starts, shown via polling) -->
  <div class="live-status" id="live-status" style="display:none">
    <span class="live-badge">● LIVE</span>
    <span id="viewer-count">0 viewers</span>
    <span id="stream-duration">00:00:00</span>
    
    <!-- Preview player (small) -->
    <video id="preview" muted autoplay></video>
    
    <button class="end-stream-btn" onclick="endStream()">End Stream</button>
  </div>
</div>
```

JavaScript:
```javascript
// Poll for stream status every 3 seconds
setInterval(async () => {
  const res = await fetch('/api/v1/live');
  const data = await res.json();
  
  // Find stream matching our stream key
  const myStream = data.streams.find(s => s.stream_key_id === MY_STREAM_KEY);
  
  if (myStream && myStream.status === 'live') {
    showLiveStatus(myStream);
    if (!previewStarted) {
      startPreviewPlayer(myStream.id);
      previewStarted = true;
    }
  } else {
    hideLiveStatus();
  }
}, 3000);
```

#### Library Page Update

Add live streams to the library grid:
- Live streams appear at the top with a red "LIVE" badge
- Show viewer count and duration
- Click → watch live page (not VOD player)
- After stream ends, the recording appears as a regular VOD

```javascript
// Updated library fetch
async function loadLibrary() {
  // Fetch both live streams and VOD videos
  const [liveRes, vodRes] = await Promise.all([
    fetch('/api/v1/live'),
    fetch('/api/v1/videos'),
  ]);
  
  const live = await liveRes.json();
  const vod = await vodRes.json();
  
  // Render live streams first, then VOD
  renderLiveStreams(live.streams);
  renderVODVideos(vod.videos);
}
```

#### Watch Live Page (`/watch-live/{stream_id}`)

Similar to the VOD player but with live-specific features:
- Red "LIVE" badge overlay
- Viewer count display
- "Back to Live" button (appears when viewer falls behind live edge)
- No seek bar (or limited seek within DVR window)
- Duration showing time since stream started

The hls.js configuration for live:
```javascript
const hls = new Hls({
  // Live-specific settings
  liveSyncDurationCount: 3,        // Target 3 segments behind live edge
  liveMaxLatencyDurationCount: 6,  // Max 6 segments behind before jumping to live
  liveDurationInfinity: true,       // Treat as infinite duration
  
  // Token setup (same as VOD)
  xhrSetup: function(xhr, url) {
    const sep = url.includes('?') ? '&' : '?';
    xhr.open('GET', url + sep + 'token=' + TOKEN, true);
  }
});

// Back to live edge button
document.getElementById('back-to-live').addEventListener('click', () => {
  hls.liveSyncPosition && (video.currentTime = hls.liveSyncPosition);
});
```

---

### 6. QoE Dashboard Update

Add live stream metrics to the existing QoE dashboard:

```go
// Add to DashboardMetrics
type DashboardMetrics struct {
    // ... existing fields ...
    
    // Live-specific
    ActiveLiveStreams  int                `json:"active_live_streams"`
    TotalLiveViewers  int                `json:"total_live_viewers"`
    PerStream         []LiveStreamMetrics `json:"per_stream"`
}

type LiveStreamMetrics struct {
    StreamID       string  `json:"stream_id"`
    Title          string  `json:"title"`
    ViewerCount    int     `json:"viewer_count"`
    AvgBitrateKbps float64 `json:"avg_bitrate_kbps"`
    RebufferRate   float64 `json:"rebuffer_rate"`
    AvgLatencyMs   int     `json:"avg_latency_ms"`
}
```

Dashboard page update:
- New "Live Streams" section at the top showing active streams, viewer counts
- Per-stream viewer count, rebuffer rate, average quality
- Everything updates via the existing SSE connection

---

## Updated Project Structure

```
video-streaming/
├── cmd/server/main.go                   # Add RTMP server startup
├── internal/
│   ├── config/config.go                 # Add RTMP_PORT
│   ├── database/postgres.go
│   ├── models/
│   │   └── models.go                    # Add StreamKey, LiveStream
│   ├── repository/
│   │   ├── video_repo.go
│   │   ├── upload_repo.go
│   │   ├── job_repo.go
│   │   ├── session_repo.go
│   │   ├── event_repo.go
│   │   ├── stream_key_repo.go           # NEW
│   │   └── live_stream_repo.go          # NEW
│   ├── service/
│   │   ├── upload_service.go
│   │   ├── transcode_service.go
│   │   ├── video_service.go
│   │   ├── session_service.go           # UPDATE: support live sessions
│   │   └── live_service.go              # NEW
│   ├── handler/
│   │   ├── upload_handler.go
│   │   ├── video_handler.go
│   │   ├── page_handler.go              # UPDATE: add go-live, watch-live pages
│   │   ├── session_handler.go           # UPDATE: support live sessions
│   │   ├── telemetry_handler.go
│   │   ├── dashboard_handler.go         # UPDATE: add live metrics
│   │   ├── stream_key_handler.go        # NEW
│   │   └── live_handler.go              # NEW (live API + HLS serving)
│   ├── middleware/
│   │   └── auth.go                      # UPDATE: support live stream tokens
│   ├── live/                             # NEW
│   │   ├── rtmp_server.go               # RTMP listener
│   │   ├── rtmp_handler.go              # RTMP connection handler
│   │   └── transcode_session.go         # Live FFmpeg management
│   ├── qoe/
│   │   └── aggregator.go               # UPDATE: add live metrics
│   ├── worker/transcode_worker.go
│   ├── transcoder/
│   │   ├── probe.go, encode.go, segment.go, manifest.go, ladder.go
│   └── web/
│       ├── templates/
│       │   ├── library.html             # UPDATE: show live streams
│       │   ├── upload.html
│       │   ├── player.html
│       │   ├── dashboard.html           # UPDATE: live section
│       │   ├── go_live.html             # NEW
│       │   └── watch_live.html          # NEW
│       └── embed.go
├── migrations/
│   ├── 001_initial.sql
│   ├── 002_sessions_and_events.sql
│   └── 003_live_streaming.sql           # NEW
├── data/
│   ├── chunks/
│   ├── raw/
│   ├── hls/                             # VOD HLS output
│   └── live/                            # NEW: live HLS output
├── docker-compose.yml
├── go.mod, go.sum
├── Makefile
└── README.md
```

## New Dependencies

```bash
go get github.com/yutopp/go-rtmp
go get github.com/yutopp/go-flv  # dependency of go-rtmp
```

If `go-rtmp` causes issues, the fallback approach is to have FFmpeg listen for RTMP directly:
```bash
# Instead of Go RTMP server, just use FFmpeg:
ffmpeg -listen 1 -i rtmp://0.0.0.0:1935/live/STREAM_KEY \
  ... (encoding flags) ... output HLS
```
In this case, write a simple TCP proxy in Go that sits in front, authenticates the stream key, and forwards to the FFmpeg listener. But try `go-rtmp` first.

## Environment Variables (Updated)

```
PORT=8080
RTMP_PORT=1935
DATABASE_URL=postgres://videostream:videostream@localhost:5432/videostream?sslmode=disable
DATA_DIR=./data
WORKER_COUNT=2
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRY=1h
```

## Server Startup (Updated main.go)

```go
func main() {
    cfg := config.Load()
    db := database.Connect(cfg.DatabaseURL)
    database.Migrate(db)
    
    // ... create repos, services, handlers ...
    
    // Start background VOD transcode workers
    jobChan := make(chan string, 100)
    for i := 0; i < cfg.WorkerCount; i++ {
        go vodWorker.Run(ctx, jobChan)
    }
    
    // Start QoE aggregator
    aggregator := qoe.NewAggregator(5 * time.Minute)
    go aggregator.Run(ctx)
    
    // Start RTMP ingest server
    rtmpServer := live.NewRTMPServer(cfg.RTMPPort, streamKeyRepo, liveService)
    go func() {
        if err := rtmpServer.Start(ctx); err != nil {
            slog.Error("rtmp_server_failed", slog.String("error", err.Error()))
        }
    }()
    
    // Re-enqueue any queued VOD jobs from previous crash
    requeuePendingJobs(db, jobChan)
    
    // Start HTTP server
    mux := http.NewServeMux()
    registerRoutes(mux, handlers, middleware)
    
    slog.Info("server_started",
        slog.Int("http_port", cfg.Port),
        slog.Int("rtmp_port", cfg.RTMPPort),
    )
    
    http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), mux)
}
```

## Testing

1. Start the server: `make dev`
2. Go to `/go-live` → create a stream key
3. Open OBS:
   - Settings → Stream → Service: Custom
   - Server: `rtmp://localhost:1935/live`
   - Stream Key: `sk_a1b2c3d4e5f6`
   - Click "Start Streaming"
4. Watch the Go Live page update to show "LIVE"
5. Open `/` (library) → see the live stream appear with red badge
6. Click it → watch the live stream with ABR
7. Open `/dashboard` → see live metrics updating
8. Open multiple browser tabs watching → see viewer count increase
9. In OBS, click "Stop Streaming" → stream ends
10. Go to library → see the recording appear as a VOD
11. Click the VOD → it plays back as a regular video

## What This Phase Teaches Me

- How RTMP ingest works (connection, publish, stream key authentication)
- How FFmpeg handles live input (continuous process, reading from pipe/RTMP)
- The critical differences between live and VOD HLS (sliding window, no ENDLIST, cache headers)
- How live streams convert to VOD recordings when they end
- How hls.js handles live playback (live sync, latency management)
- The full live pipeline: OBS → RTMP → FFmpeg → HLS → CDN-like serving → player

## Important Notes

- The RTMP library (`go-rtmp`) may have API differences from what I've shown — adapt to its actual interface
- If RTMP causes issues, the fallback is TCP proxy + FFmpeg RTMP listener (described above)
- Named pipes (FIFOs) may not work on all OS — alternative is to use Go `io.Pipe()` or have FFmpeg pull from RTMP
- Live segments are 2 seconds (vs 4 for VOD) — this is intentional for lower latency
- Old segments are deleted by FFmpeg (`delete_segments` flag) — the sliding window keeps only 5 segments
- For VOD conversion: we just add `#EXT-X-ENDLIST` to existing playlists — no re-transcode needed
- Test with a short stream first (30 seconds) to verify the full pipeline before long sessions