# Claude Code Prompt: Video Streaming Platform — Phase 1

## Context

I'm building a video streaming platform from scratch as a learning project. This is Phase 1: the foundational pipeline that takes a video file, transcodes it into multiple qualities, segments it for HLS streaming, and serves it with adaptive bitrate playback in the browser.

I'm a senior backend engineer experienced in Go (monolith, 3-layer architecture: handler/service/repository), but new to video engineering.

## What To Build

A Go project with two commands and a web player:

### 1. CLI: `go run cmd/transcode/main.go -input video.mp4 -output ./output`

This command takes an input video file and produces HLS-ready output:

**Step 1 — Probe the source video using FFprobe:**
- Extract: resolution, codec, framerate, duration, bitrate
- Print a summary to stdout
- Use FFprobe JSON output (`-print_format json -show_streams -show_format`)

**Step 2 — Build the encoding ladder:**
- Define 3 quality levels (never upscale past source resolution):
  - 720p: 1280x720, 2500kbps video, 128kbps audio
  - 480p: 854x480, 1000kbps video, 128kbps audio
  - 360p: 640x360, 600kbps video, 96kbps audio
- Skip any quality level higher than the source resolution

**Step 3 — Transcode each quality level using FFmpeg:**
- Codec: H.264 (libx264), AAC audio
- Preset: `medium` (this is a learning project, not live)
- GOP: 120 frames at 30fps = 4 seconds (must match segment duration)
- Key flags: `-g 120 -keyint_min 120 -sc_threshold 0 -bf 2`
- Use `-movflags +faststart` on intermediate MP4s

**Step 4 — Segment each quality level into HLS fMP4:**
- Segment duration: 4 seconds
- Use FFmpeg HLS muxer: `-f hls -hls_time 4 -hls_playlist_type vod -hls_segment_type fmp4`
- Each quality gets its own directory with `playlist.m3u8` + `init.mp4` + `segment_*.m4s`

**Step 5 — Generate the master playlist:**
- Create `master.m3u8` at the output root
- Each variant entry includes: BANDWIDTH, RESOLUTION, CODECS (use `avc1.64001f,mp4a.40.2` for H.264+AAC)
- Order from highest to lowest bandwidth

**Output directory structure:**
```
output/
├── master.m3u8
├── 720p/
│   ├── playlist.m3u8
│   ├── init.mp4
│   ├── segment_0000.m4s
│   ├── segment_0001.m4s
│   └── ...
├── 480p/
│   ├── playlist.m3u8
│   ├── init.mp4
│   └── segment_*.m4s
└── 360p/
    ├── playlist.m3u8
    ├── init.mp4
    └── segment_*.m4s
```

### 2. HTTP Server: `go run cmd/server/main.go -dir ./output -port 8080`

A simple Go HTTP server that serves the transcoded output:

- Serve static files from the output directory
- Set correct Content-Type headers:
  - `.m3u8` → `application/vnd.apple.mpegurl`
  - `.m4s` → `video/iso.bmff`
  - `.mp4` → `video/mp4`
- Enable CORS (Access-Control-Allow-Origin: *)
- Serve the web player at `/` (embedded HTML)
- Serve video files at `/videos/**`
- Log each request with the path and response time using `slog`

### 3. Web Player (embedded in the Go server as an HTML string or embedded file)

A single HTML page using hls.js that:

- Loads hls.js from CDN: `https://cdn.jsdelivr.net/npm/hls.js@latest`
- Has a `<video>` element that plays the master playlist
- Shows a **quality indicator overlay** in the corner displaying:
  - Current quality level (e.g., "720p")
  - Current measured bandwidth
  - Buffer level (seconds ahead)
  - Segment download time
- Has a **manual quality selector** dropdown that lets the user force a specific quality or set to "Auto" (ABR)
- Listens to hls.js events:
  - `Hls.Events.LEVEL_SWITCHED` — update quality display
  - `Hls.Events.FRAG_LOADED` — update bandwidth/segment stats
  - `Hls.Events.BUFFER_APPENDED` — update buffer level
- Style it cleanly with minimal CSS (dark background, video centered)

## Project Structure

```
video-streaming/
├── cmd/
│   ├── transcode/
│   │   └── main.go          # CLI entry point
│   └── server/
│       └── main.go          # HTTP server entry point
├── internal/
│   ├── transcoder/
│   │   ├── probe.go         # FFprobe wrapper
│   │   ├── encode.go        # FFmpeg transcoding
│   │   ├── segment.go       # HLS segmentation
│   │   ├── manifest.go      # Master playlist generation
│   │   └── ladder.go        # Encoding ladder definition + filtering
│   └── server/
│       ├── handler.go       # HTTP handlers
│       └── player.go        # Embedded HTML player
├── go.mod
├── go.sum
├── Makefile                  # Convenience commands
└── README.md
```

## Technical Requirements

- **Go 1.21+** with standard library only (no web frameworks, no video libraries — just `os/exec` for FFmpeg)
- **FFmpeg and FFprobe** must be installed on the system (assume they're available in PATH)
- Use `os/exec.CommandContext` for running FFmpeg with proper context cancellation
- Use `slog` for structured logging throughout
- Use `encoding/json` for parsing FFprobe output
- Parse and display FFmpeg progress (read stderr for frame/fps/time output)
- Handle errors properly — if FFmpeg fails, capture stderr and include it in the error message
- The transcode CLI should show progress for each encoding step

## Encoding Ladder Definition

```go
type Profile struct {
    Name       string // "720p", "480p", "360p"
    Width      int    // 1280, 854, 640
    Height     int    // 720, 480, 360
    VideoBitrate string // "2500k", "1000k", "600k"
    AudioBitrate string // "128k", "128k", "96k"
    MaxRate    string // "2750k", "1100k", "660k" (1.1x video bitrate)
    BufSize    string // "5000k", "2000k", "1200k" (2x video bitrate)
}
```

## Key FFmpeg Commands To Generate

For transcoding (per quality level):
```bash
ffmpeg -i input.mp4 \
  -c:v libx264 -preset medium \
  -b:v 2500k -maxrate 2750k -bufsize 5000k \
  -vf scale=1280:720 \
  -g 120 -keyint_min 120 -sc_threshold 0 -bf 2 \
  -c:a aac -b:a 128k -ar 48000 \
  -movflags +faststart \
  output/720p/intermediate.mp4
```

For segmentation (per quality level):
```bash
ffmpeg -i output/720p/intermediate.mp4 \
  -c copy \
  -f hls \
  -hls_time 4 \
  -hls_playlist_type vod \
  -hls_segment_type fmp4 \
  -hls_segment_filename 'output/720p/segment_%04d.m4s' \
  -hls_fmp4_init_filename 'init.mp4' \
  output/720p/playlist.m3u8
```

## Master Playlist Format

```m3u8
#EXTM3U
#EXT-X-VERSION:7

#EXT-X-STREAM-INF:BANDWIDTH=2628000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=1128000,RESOLUTION=854x480,CODECS="avc1.64001f,mp4a.40.2"
480p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=696000,RESOLUTION=640x360,CODECS="avc1.64001f,mp4a.40.2"
360p/playlist.m3u8
```

## What I Want To Learn From This

- How FFmpeg transcoding actually works (flags, presets, GOP config)
- How HLS segmentation produces fMP4 segments and playlists
- How a master playlist connects multiple quality levels
- How hls.js implements ABR in the browser
- How segment requests flow from player to server
- The relationship between GOP size and segment duration

## Important Notes

- Do NOT use any third-party Go libraries for video processing — only `os/exec` to call FFmpeg/FFprobe
- Keep the code clean with proper separation of concerns (probe/encode/segment/manifest as separate functions)
- Include helpful comments explaining the "why" behind FFmpeg flags
- The Makefile should have: `make transcode INPUT=video.mp4` and `make serve`
- Add a README explaining how to use it and what each component does
- If FFmpeg is not found in PATH, print a helpful error message explaining how to install it
