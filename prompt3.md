# Claude Code Prompt: Video Streaming Platform — Phase 3

## Context

I'm building a video streaming platform from scratch. I've completed:

- **Phase 1:** Go CLI that transcodes video into multi-quality HLS (720p/480p/360p) using FFmpeg, HTTP server serving segments, hls.js player with ABR
- **Phase 2:** Chunked resumable upload API, PostgreSQL for video metadata/jobs, background transcode worker pool (channel-based), web UI with upload page + video library + player

The existing project structure:
```
video-streaming/
├── cmd/server/main.go
├── internal/
│   ├── config/config.go
│   ├── database/postgres.go
│   ├── models/models.go
│   ├── repository/
│   │   ├── video_repo.go
│   │   ├── upload_repo.go
│   │   └── job_repo.go
│   ├── service/
│   │   ├── upload_service.go
│   │   ├── transcode_service.go
│   │   └── video_service.go
│   ├── handler/
│   │   ├── upload_handler.go
│   │   ├── video_handler.go
│   │   └── page_handler.go
│   ├── worker/transcode_worker.go
│   ├── transcoder/
│   │   ├── probe.go, encode.go, segment.go, manifest.go, ladder.go
│   └── web/
│       ├── templates/ (library.html, upload.html, player.html)
│       └── embed.go
├── migrations/001_initial.sql
├── data/ (chunks/, raw/, hls/)
├── docker-compose.yml
├── go.mod, Makefile, README.md
```

Now I'm building Phase 3: the delivery layer that secures video access, tracks playback quality, and surfaces QoE metrics in a dashboard.

## What To Build

### Overview

Three new capabilities layered on top of the existing platform:

1. **Playback Sessions + Signed URLs** — Player must create a session before watching. All segment requests require a valid JWT token. No token = no video.
2. **Client Telemetry Pipeline** — The hls.js player reports real-time playback metrics (TTFF, rebuffer, bandwidth, quality switches) via Server-Sent Events (SSE) for live dashboard updates and POST endpoint for batched events.
3. **QoE Dashboard** — A real-time dashboard page showing platform health: active sessions, TTFF distribution, rebuffer rate, average bitrate, per-video breakdowns.

---

### 1. Playback Sessions + Signed URLs

#### New Database Tables

```sql
CREATE TABLE playback_sessions (
    id              TEXT PRIMARY KEY,
    video_id        TEXT NOT NULL REFERENCES videos(id),
    token           TEXT NOT NULL,               -- JWT token issued for this session
    device_type     TEXT,                        -- 'desktop', 'mobile', 'tablet'
    user_agent      TEXT,
    ip_address      TEXT,
    started_at      TIMESTAMP DEFAULT NOW(),
    last_active_at  TIMESTAMP DEFAULT NOW(),     -- updated on each segment request
    ended_at        TIMESTAMP,                   -- set when player sends 'ended' event
    status          TEXT DEFAULT 'active'         -- active, ended, expired
);

CREATE TABLE playback_events (
    id              BIGSERIAL PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES playback_sessions(id),
    video_id        TEXT NOT NULL,
    event_type      TEXT NOT NULL,               -- see event types below
    timestamp       TIMESTAMP DEFAULT NOW(),

    -- Segment metrics (for 'segment_downloaded' events)
    segment_number  INTEGER,
    segment_quality TEXT,                        -- '720p', '480p', '360p'
    segment_bytes   BIGINT,
    download_time_ms INTEGER,
    throughput_bps  BIGINT,

    -- Playback state (for 'heartbeat' and state-change events)
    current_quality TEXT,
    buffer_length   FLOAT,                      -- seconds of buffer ahead
    playback_position FLOAT,                    -- current time in seconds
    
    -- Experience events
    rebuffer_duration_ms INTEGER,               -- for 'rebuffer_end' events
    quality_from    TEXT,                        -- for 'quality_change' events
    quality_to      TEXT,

    -- Error info
    error_code      TEXT,
    error_message   TEXT
);

-- Index for real-time dashboard queries
CREATE INDEX idx_playback_events_session ON playback_events(session_id);
CREATE INDEX idx_playback_events_type_time ON playback_events(event_type, timestamp);
CREATE INDEX idx_playback_events_video ON playback_events(video_id, timestamp);
```

#### Event Types
```
playback_start      — Player started playback (first frame rendered)
segment_downloaded  — A segment was fetched (includes timing data)
quality_change      — ABR switched quality level
rebuffer_start      — Player stalled (buffer empty)
rebuffer_end        — Playback resumed after stall (includes duration)
heartbeat           — Periodic status update (every 5 seconds)
seek                — User jumped to new position
playback_end        — Video finished or user navigated away
error               — Something went wrong
```

#### Session API

**POST /api/v1/videos/{video_id}/sessions** — Create playback session
```json
// Request
{
  "device_type": "desktop",
  "user_agent": "Mozilla/5.0..."
}

// Response
{
  "session_id": "sess_abc123",
  "manifest_url": "/videos/abc123/master.m3u8?token=eyJhbG...",
  "token": "eyJhbGciOiJIUzI1NiJ9...",
  "token_expires_at": "2025-01-15T11:00:00Z",
  "telemetry_url": "/api/v1/sessions/sess_abc123/events"
}
```

Implementation:
1. Verify video exists and status = "ready"
2. Create playback_session record
3. Generate JWT token with claims:
   ```go
   type PlaybackClaims struct {
       jwt.RegisteredClaims
       SessionID string `json:"sid"`
       VideoID   string `json:"vid"`
   }
   ```
4. Token expires in 1 hour (configurable)
5. Return manifest URL with token as query parameter

**JWT signing:** Use `github.com/golang-jwt/jwt/v5`. Signing key from env var `JWT_SECRET` (generate a random one in Makefile or README).

#### Signed URL Middleware

Create middleware that validates JWT on ALL segment/manifest requests:

```go
// Apply to all routes under /videos/{video_id}/
func (m *AuthMiddleware) ValidatePlaybackToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.URL.Query().Get("token")
        if token == "" {
            http.Error(w, "missing token", http.StatusUnauthorized)
            return
        }

        claims, err := m.parseAndValidate(token)
        if err != nil {
            http.Error(w, "invalid token", http.StatusForbidden)
            return
        }

        // Verify the requested video matches the token's video_id
        requestedVideoID := extractVideoIDFromPath(r.URL.Path)
        if claims.VideoID != requestedVideoID {
            http.Error(w, "token not valid for this video", http.StatusForbidden)
            return
        }

        // Update session last_active_at (debounce: only update every 30s)
        go m.touchSession(claims.SessionID)

        // Add claims to context for downstream handlers
        ctx := context.WithValue(r.Context(), "claims", claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**IMPORTANT for hls.js:** When hls.js loads a master playlist, it will follow the variant playlist URLs inside. Those internal URLs won't have the token. There are two approaches:

**Approach A (simpler, recommended):** Use the hls.js `xhrSetup` config to append the token to every request:
```javascript
const hls = new Hls({
  xhrSetup: function(xhr, url) {
    // Append token to every segment/playlist request
    const separator = url.includes('?') ? '&' : '?';
    xhr.open('GET', url + separator + 'token=' + TOKEN, true);
  }
});
```

**Approach B:** Rewrite playlist URLs server-side to include the token (more complex, do NOT use this approach).

Use Approach A.

#### Route Structure Update

```
# Public routes (no auth)
GET  /                                    → Library page
GET  /upload                              → Upload page  
GET  /watch/{video_id}                    → Player page (page itself is public)
POST /api/v1/uploads                      → Init upload
PUT  /api/v1/uploads/{id}/chunks/{n}      → Upload chunk
GET  /api/v1/uploads/{id}/status          → Upload status
GET  /api/v1/videos                       → List videos
GET  /api/v1/videos/{id}                  → Video details
POST /api/v1/videos/{id}/sessions         → Create session (returns token)

# Protected routes (require valid JWT token)
GET  /videos/{video_id}/master.m3u8       → Master playlist
GET  /videos/{video_id}/{quality}/*       → Variant playlists + segments

# Telemetry routes (require valid session_id)
POST /api/v1/sessions/{session_id}/events → Batch post events
GET  /api/v1/sessions/{session_id}/events/stream → SSE stream (for dashboard, not player)

# Dashboard (no auth, read-only)
GET  /dashboard                           → QoE dashboard page
GET  /api/v1/dashboard/stats              → Real-time aggregated metrics
GET  /api/v1/dashboard/stats/stream       → SSE stream of live metrics
```

---

### 2. Client Telemetry Pipeline

#### Player-Side Telemetry (update player.html JavaScript)

The hls.js player must report events back to the server. Implement a telemetry client in the player JavaScript:

```javascript
class TelemetryClient {
  constructor(sessionId, telemetryUrl) {
    this.sessionId = sessionId;
    this.telemetryUrl = telemetryUrl;
    this.eventBuffer = [];
    this.flushInterval = setInterval(() => this.flush(), 3000); // flush every 3s
    this.playbackStartTime = null;  // for TTFF calculation
  }

  // Called when user clicks play
  recordPlayRequested() {
    this.playbackStartTime = performance.now();
  }

  // Called on Hls.Events.FRAG_LOADED
  recordSegmentDownloaded(data) {
    this.push({
      event_type: 'segment_downloaded',
      segment_number: data.frag.sn,
      segment_quality: this.getQualityLabel(data.frag.level),
      segment_bytes: data.frag.loaded,
      download_time_ms: Math.round(data.frag.stats.loading.end - data.frag.stats.loading.start),
      throughput_bps: Math.round(data.frag.loaded * 8 / 
        ((data.frag.stats.loading.end - data.frag.stats.loading.start) / 1000)),
    });
  }

  // Called on first video 'playing' event
  recordPlaybackStart(bufferLength) {
    const ttfm = this.playbackStartTime 
      ? Math.round(performance.now() - this.playbackStartTime) 
      : null;
    this.push({
      event_type: 'playback_start',
      buffer_length: bufferLength,
      download_time_ms: ttfm,  // reuse field for TTFF
    });
  }

  // Called on Hls.Events.LEVEL_SWITCHED
  recordQualityChange(fromLevel, toLevel) {
    this.push({
      event_type: 'quality_change',
      quality_from: this.getQualityLabel(fromLevel),
      quality_to: this.getQualityLabel(toLevel),
    });
  }

  // Called when video.waiting fires
  recordRebufferStart() {
    this._rebufferStartTime = performance.now();
    this.push({ event_type: 'rebuffer_start' });
  }

  // Called when video.playing fires after waiting
  recordRebufferEnd() {
    const duration = this._rebufferStartTime 
      ? Math.round(performance.now() - this._rebufferStartTime) 
      : 0;
    this.push({ event_type: 'rebuffer_end', rebuffer_duration_ms: duration });
    this._rebufferStartTime = null;
  }

  // Periodic heartbeat (call every 5 seconds)
  recordHeartbeat(videoElement, hls) {
    this.push({
      event_type: 'heartbeat',
      current_quality: this.getQualityLabel(hls.currentLevel),
      buffer_length: this.getBufferLength(videoElement),
      playback_position: videoElement.currentTime,
    });
  }

  // Called on video 'ended' or page unload
  recordPlaybackEnd() {
    this.push({ event_type: 'playback_end' });
    this.flush(); // immediate flush
  }

  push(event) {
    event.timestamp = new Date().toISOString();
    this.eventBuffer.push(event);
  }

  async flush() {
    if (this.eventBuffer.length === 0) return;
    const events = this.eventBuffer.splice(0);
    try {
      await fetch(this.telemetryUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ events }),
      });
    } catch (e) {
      // Put events back on failure (best effort)
      this.eventBuffer.unshift(...events);
    }
  }

  destroy() {
    clearInterval(this.flushInterval);
    this.recordPlaybackEnd();
  }
}
```

#### Server-Side Telemetry Handler

**POST /api/v1/sessions/{session_id}/events** — Receive batched events
```json
// Request
{
  "events": [
    { "event_type": "segment_downloaded", "segment_number": 5, "segment_quality": "720p", "segment_bytes": 2500000, "download_time_ms": 312, "throughput_bps": 64102564, "timestamp": "..." },
    { "event_type": "heartbeat", "current_quality": "720p", "buffer_length": 12.5, "playback_position": 23.4, "timestamp": "..." }
  ]
}

// Response
{ "received": 2 }
```

Implementation:
1. Validate session_id exists and is active
2. Batch insert all events into playback_events table
3. Forward events to the in-memory QoE aggregator (for real-time dashboard)
4. Return count of received events

```go
// handler
func (h *TelemetryHandler) PostEvents(w http.ResponseWriter, r *http.Request) {
    sessionID := r.PathValue("session_id")
    
    var req struct {
        Events []models.PlaybackEvent `json:"events"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // Set session_id and video_id on each event
    session, _ := h.sessionRepo.Get(r.Context(), sessionID)
    for i := range req.Events {
        req.Events[i].SessionID = sessionID
        req.Events[i].VideoID = session.VideoID
    }
    
    // Batch insert to DB
    h.eventRepo.BatchInsert(r.Context(), req.Events)
    
    // Forward to real-time aggregator
    h.qoeAggregator.Ingest(req.Events)
    
    writeJSON(w, map[string]int{"received": len(req.Events)})
}
```

---

### 3. QoE Aggregator + Dashboard

#### In-Memory QoE Aggregator

This component keeps a sliding window (last 5 minutes) of aggregated metrics in memory. It's fed by the telemetry handler and read by the dashboard API.

```go
// internal/qoe/aggregator.go

type Aggregator struct {
    mu              sync.RWMutex
    windowDuration  time.Duration  // 5 minutes
    
    // Global metrics
    activeSessions  map[string]time.Time  // session_id → last heartbeat time
    
    // Sliding window of events for computation
    recentEvents    []timedEvent
    
    // Pre-computed metrics (recalculated every second)
    currentMetrics  *DashboardMetrics
    
    // SSE subscribers for live updates
    subscribers     map[chan *DashboardMetrics]struct{}
    subMu           sync.RWMutex
}

type DashboardMetrics struct {
    Timestamp         time.Time `json:"timestamp"`
    
    // Global health
    ActiveSessions    int       `json:"active_sessions"`
    TotalSessionsLast5m int     `json:"total_sessions_5m"`
    
    // TTFF (Time To First Frame)
    TTFFMedianMs      int       `json:"ttff_median_ms"`
    TTFFP95Ms         int       `json:"ttff_p95_ms"`
    
    // Rebuffering
    RebufferRate      float64   `json:"rebuffer_rate"`       // % of sessions with at least 1 rebuffer
    AvgRebufferDurationMs int   `json:"avg_rebuffer_duration_ms"`
    
    // Quality
    AvgBitrateKbps    float64   `json:"avg_bitrate_kbps"`
    QualityDistribution map[string]float64 `json:"quality_distribution"` // {"720p": 0.45, "480p": 0.35, "360p": 0.20}
    QualitySwitchesPerMin float64 `json:"quality_switches_per_min"`
    
    // Throughput
    AvgThroughputMbps float64   `json:"avg_throughput_mbps"`
    P10ThroughputMbps float64   `json:"p10_throughput_mbps"`  // worst 10%
    
    // Per-video breakdown (top 10 by active sessions)
    PerVideo          []VideoMetrics `json:"per_video"`
}

type VideoMetrics struct {
    VideoID        string  `json:"video_id"`
    Title          string  `json:"title"`
    ActiveSessions int     `json:"active_sessions"`
    AvgBitrateKbps float64 `json:"avg_bitrate_kbps"`
    RebufferRate   float64 `json:"rebuffer_rate"`
}
```

Aggregator methods:
```go
func (a *Aggregator) Ingest(events []models.PlaybackEvent)  // called by telemetry handler
func (a *Aggregator) GetMetrics() *DashboardMetrics          // called by dashboard API
func (a *Aggregator) Subscribe() chan *DashboardMetrics       // SSE subscription
func (a *Aggregator) Unsubscribe(ch chan *DashboardMetrics)   // SSE cleanup
```

The aggregator runs a background goroutine that:
1. Every 1 second: recomputes `currentMetrics` from `recentEvents`
2. Every 1 second: pushes `currentMetrics` to all SSE subscribers
3. Every 10 seconds: prunes events older than `windowDuration` (5 minutes)
4. Every 30 seconds: prunes sessions with no heartbeat for 60 seconds (mark as "ended")

#### Dashboard API

**GET /api/v1/dashboard/stats** — Get current metrics snapshot
```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "active_sessions": 3,
  "total_sessions_5m": 7,
  "ttff_median_ms": 850,
  "ttff_p95_ms": 2100,
  "rebuffer_rate": 0.14,
  "avg_rebuffer_duration_ms": 1200,
  "avg_bitrate_kbps": 2100,
  "quality_distribution": { "720p": 0.45, "480p": 0.35, "360p": 0.20 },
  "quality_switches_per_min": 0.8,
  "avg_throughput_mbps": 8.5,
  "p10_throughput_mbps": 2.1,
  "per_video": [
    { "video_id": "abc", "title": "My Video", "active_sessions": 2, "avg_bitrate_kbps": 2300, "rebuffer_rate": 0.0 }
  ]
}
```

**GET /api/v1/dashboard/stats/stream** — SSE stream of live metrics
```
data: {"timestamp":"...","active_sessions":3,...}

data: {"timestamp":"...","active_sessions":4,...}
```

SSE handler:
```go
func (h *DashboardHandler) StatsStream(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    ch := h.aggregator.Subscribe()
    defer h.aggregator.Unsubscribe(ch)
    
    for {
        select {
        case metrics := <-ch:
            data, _ := json.Marshal(metrics)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

#### Dashboard Web Page (`/dashboard`)

A single HTML page with real-time updating charts and metrics. Use vanilla JavaScript + SSE + minimal CSS. No chart libraries needed — use simple HTML/CSS bar charts and number displays.

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│  QoE Dashboard                              Live ● (green)  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │ Active   │ │ TTFF     │ │ Rebuffer │ │ Avg      │      │
│  │ Sessions │ │ Median   │ │ Rate     │ │ Bitrate  │      │
│  │    3     │ │  850ms   │ │  14%     │ │ 2.1 Mbps │      │
│  │          │ │ p95:2.1s │ │          │ │          │      │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
│                                                             │
│  Quality Distribution          Throughput                   │
│  ┌────────────────────┐       ┌────────────────────┐       │
│  │ 720p ████████ 45%  │       │ Avg: 8.5 Mbps      │       │
│  │ 480p ██████  35%   │       │ p10: 2.1 Mbps      │       │
│  │ 360p ████   20%    │       │                     │       │
│  └────────────────────┘       └────────────────────┘       │
│                                                             │
│  Per-Video Breakdown                                        │
│  ┌──────────────────────────────────────────────────┐      │
│  │ Video              │ Viewers │ Bitrate │ Rebuffer │      │
│  │ My Video           │   2     │ 2.3Mbps│   0%     │      │
│  │ Tutorial           │   1     │ 1.8Mbps│  33%     │      │
│  └──────────────────────────────────────────────────┘      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

Dashboard JavaScript:
```javascript
// Connect to SSE stream
const evtSource = new EventSource('/api/v1/dashboard/stats/stream');
evtSource.onmessage = function(event) {
  const metrics = JSON.parse(event.data);
  updateDashboard(metrics);
};

function updateDashboard(m) {
  // Update metric cards
  document.getElementById('active-sessions').textContent = m.active_sessions;
  document.getElementById('ttff-median').textContent = m.ttff_median_ms + 'ms';
  document.getElementById('ttff-p95').textContent = 'p95: ' + (m.ttff_p95_ms / 1000).toFixed(1) + 's';
  document.getElementById('rebuffer-rate').textContent = (m.rebuffer_rate * 100).toFixed(0) + '%';
  document.getElementById('avg-bitrate').textContent = (m.avg_bitrate_kbps / 1000).toFixed(1) + ' Mbps';
  
  // Update quality distribution bars
  updateQualityBars(m.quality_distribution);
  
  // Update per-video table
  updateVideoTable(m.per_video);
  
  // Color-code rebuffer rate: green <1%, yellow 1-5%, red >5%
  colorCodeMetric('rebuffer-rate', m.rebuffer_rate, [0.01, 0.05]);
  
  // Color-code TTFF: green <1000ms, yellow 1000-2000ms, red >2000ms
  colorCodeMetric('ttff-median', m.ttff_median_ms, [1000, 2000]);
}
```

---

### 4. Updated Player Integration

The player page (`/watch/{video_id}`) needs significant updates:

**New playback flow:**
```
1. Page loads with video_id
2. JavaScript calls POST /api/v1/videos/{video_id}/sessions
   → Gets session_id, token, manifest_url, telemetry_url
3. Initialize hls.js with token-appending xhrSetup
4. Initialize TelemetryClient with session_id and telemetry_url
5. Wire up hls.js events to TelemetryClient methods
6. Start heartbeat interval (every 5 seconds)
7. On page unload: call telemetry.destroy() to flush final events
```

**Updated player JavaScript structure:**
```javascript
async function initPlayer(videoId) {
  // 1. Create session
  const session = await fetch(`/api/v1/videos/${videoId}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ device_type: detectDeviceType() }),
  }).then(r => r.json());

  const TOKEN = session.token;
  const MANIFEST = session.manifest_url;

  // 2. Init telemetry
  const telemetry = new TelemetryClient(session.session_id, session.telemetry_url);

  // 3. Init hls.js with signed URL support
  const video = document.getElementById('video');
  const hls = new Hls({
    xhrSetup: function(xhr, url) {
      const sep = url.includes('?') ? '&' : '?';
      xhr.open('GET', url + sep + 'token=' + TOKEN, true);
    }
  });

  // 4. Wire up events
  telemetry.recordPlayRequested();

  hls.on(Hls.Events.FRAG_LOADED, (event, data) => {
    telemetry.recordSegmentDownloaded(data);
    updateStatsOverlay(data);
  });

  hls.on(Hls.Events.LEVEL_SWITCHED, (event, data) => {
    const fromLevel = hls.previousLevel || -1;
    telemetry.recordQualityChange(fromLevel, data.level);
    updateQualityDisplay(hls.levels[data.level]);
  });

  video.addEventListener('playing', () => {
    if (!telemetry._started) {
      telemetry.recordPlaybackStart(getBufferLength(video));
      telemetry._started = true;
    }
    if (telemetry._rebuffering) {
      telemetry.recordRebufferEnd();
      telemetry._rebuffering = false;
    }
  });

  video.addEventListener('waiting', () => {
    if (telemetry._started) {
      telemetry.recordRebufferStart();
      telemetry._rebuffering = true;
    }
  });

  video.addEventListener('ended', () => telemetry.recordPlaybackEnd());

  // 5. Heartbeat
  setInterval(() => telemetry.recordHeartbeat(video, hls), 5000);

  // 6. Cleanup on page unload
  window.addEventListener('beforeunload', () => telemetry.destroy());

  // 7. Start playback
  hls.loadSource(MANIFEST);
  hls.attachMedia(video);
}
```

---

## Updated Project Structure

```
video-streaming/
├── cmd/server/main.go
├── internal/
│   ├── config/config.go          # Add JWT_SECRET
│   ├── database/postgres.go
│   ├── models/
│   │   └── models.go             # Add PlaybackSession, PlaybackEvent
│   ├── repository/
│   │   ├── video_repo.go
│   │   ├── upload_repo.go
│   │   ├── job_repo.go
│   │   ├── session_repo.go       # NEW
│   │   └── event_repo.go         # NEW (with BatchInsert)
│   ├── service/
│   │   ├── upload_service.go
│   │   ├── transcode_service.go
│   │   ├── video_service.go
│   │   └── session_service.go    # NEW (create session, generate JWT)
│   ├── handler/
│   │   ├── upload_handler.go
│   │   ├── video_handler.go
│   │   ├── page_handler.go
│   │   ├── session_handler.go    # NEW
│   │   ├── telemetry_handler.go  # NEW
│   │   └── dashboard_handler.go  # NEW (stats API + SSE)
│   ├── middleware/
│   │   └── auth.go               # NEW (JWT validation middleware)
│   ├── qoe/
│   │   └── aggregator.go         # NEW (in-memory metrics aggregation)
│   ├── worker/transcode_worker.go
│   ├── transcoder/
│   │   ├── probe.go, encode.go, segment.go, manifest.go, ladder.go
│   └── web/
│       ├── templates/
│       │   ├── library.html
│       │   ├── upload.html
│       │   ├── player.html       # UPDATED with session init + telemetry
│       │   └── dashboard.html    # NEW
│       └── embed.go
├── migrations/
│   ├── 001_initial.sql
│   └── 002_sessions_and_events.sql  # NEW
├── data/
├── docker-compose.yml
├── go.mod, go.sum
├── Makefile
└── README.md
```

## New Dependencies

```
go get github.com/golang-jwt/jwt/v5
```

No other new dependencies. Everything else uses the standard library.

## Environment Variables (Updated)

```
PORT=8080
DATABASE_URL=postgres://videostream:videostream@localhost:5432/videostream?sslmode=disable
DATA_DIR=./data
WORKER_COUNT=2
JWT_SECRET=your-secret-key-change-in-production   # min 32 chars
JWT_EXPIRY=1h
```

## Key Implementation Details

### JWT Token Generation
```go
func (s *SessionService) GenerateToken(session *models.PlaybackSession) (string, time.Time, error) {
    expiresAt := time.Now().Add(s.tokenExpiry)
    
    claims := PlaybackClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(expiresAt),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ID:        session.ID,
        },
        SessionID: session.ID,
        VideoID:   session.VideoID,
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString([]byte(s.jwtSecret))
    return signed, expiresAt, err
}
```

### Batch Insert for Events (performance critical)
```go
func (r *EventRepo) BatchInsert(ctx context.Context, events []models.PlaybackEvent) error {
    // Use a single INSERT with multiple VALUE tuples
    // Much faster than inserting one at a time
    
    query := "INSERT INTO playback_events (session_id, video_id, event_type, timestamp, segment_number, segment_quality, segment_bytes, download_time_ms, throughput_bps, current_quality, buffer_length, playback_position, rebuffer_duration_ms, quality_from, quality_to, error_code, error_message) VALUES "
    
    var values []string
    var args []interface{}
    argIdx := 1
    
    for _, e := range events {
        values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
            argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6,
            argIdx+7, argIdx+8, argIdx+9, argIdx+10, argIdx+11, argIdx+12,
            argIdx+13, argIdx+14, argIdx+15, argIdx+16))
        args = append(args, e.SessionID, e.VideoID, e.EventType, e.Timestamp,
            e.SegmentNumber, e.SegmentQuality, e.SegmentBytes,
            e.DownloadTimeMs, e.ThroughputBps, e.CurrentQuality,
            e.BufferLength, e.PlaybackPosition, e.RebufferDurationMs,
            e.QualityFrom, e.QualityTo, e.ErrorCode, e.ErrorMessage)
        argIdx += 17
    }
    
    query += strings.Join(values, ",")
    _, err := r.db.ExecContext(ctx, query, args...)
    return err
}
```

### QoE Aggregator Percentile Calculation
```go
// Simple percentile for TTFF calculation
func percentile(sorted []int, pct float64) int {
    if len(sorted) == 0 {
        return 0
    }
    idx := int(float64(len(sorted)-1) * pct)
    return sorted[idx]
}

// Usage in recalculate:
sort.Ints(ttffValues)
metrics.TTFFMedianMs = percentile(ttffValues, 0.50)
metrics.TTFFP95Ms = percentile(ttffValues, 0.95)
```

### Logging (slog throughout)
Every handler and service method should log with relevant context:
```go
slog.Info("session_created",
    slog.String("session_id", session.ID),
    slog.String("video_id", session.VideoID),
    slog.String("device_type", session.DeviceType),
    slog.String("ip", session.IPAddress),
)

slog.Info("events_received",
    slog.String("session_id", sessionID),
    slog.Int("event_count", len(events)),
)
```

## What This Phase Teaches Me

- How signed URLs protect video content (JWT on every segment request)
- How hls.js is configured to pass tokens with every request
- How client-side telemetry captures real playback quality metrics
- How Server-Sent Events (SSE) enable real-time dashboard updates
- How in-memory aggregation works for low-latency metric computation
- How batch inserts optimize high-volume event ingestion
- The full observability loop: player → telemetry → aggregator → dashboard

## Important Notes

- Reuse all existing Phase 1 + Phase 2 code — don't rewrite the transcoder, upload, or worker
- The JWT middleware ONLY applies to /videos/** routes (segment serving), not to the API or pages
- The dashboard should work with zero sessions (show zeros, not errors)
- Handle the case where hls.js is not supported (Safari native HLS won't use xhrSetup — for this learning project, note the limitation in README but don't solve it)
- The QoE aggregator is in-memory only — it resets on server restart. That's fine for now.
- Test by opening multiple browser tabs playing different videos and watching the dashboard update in real-time