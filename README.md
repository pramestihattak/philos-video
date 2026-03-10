# philos-video

A minimal HLS transcoding pipeline written in Go, for learning video engineering.

## Architecture

```
cmd/transcode/   CLI: probe → encode → segment → manifest
cmd/server/      HTTP server: static HLS files + embedded hls.js player

internal/transcoder/
  ladder.go      Encoding ladder (720p / 480p / 360p)
  probe.go       ffprobe wrapper → VideoInfo
  encode.go      FFmpeg x264 encode → intermediate.mp4
  segment.go     FFmpeg fMP4 HLS segmentation
  manifest.go    master.m3u8 generator

internal/server/
  player.go      Embedded HTML + hls.js player
  handler.go     HTTP mux, CORS middleware, MIME types
```

## Requirements

- Go 1.21+
- FFmpeg (with libx264 and AAC support)

Install FFmpeg:
- macOS: `brew install ffmpeg`
- Ubuntu: `sudo apt install ffmpeg`
- Windows: https://ffmpeg.org/download.html

## Usage

### Transcode a video

```bash
make transcode INPUT=video.mp4
# or with custom output dir:
make transcode INPUT=video.mp4 OUTPUT=./out
```

Equivalent direct invocation:
```bash
go run cmd/transcode/main.go -input video.mp4 -output ./output
```

This produces:
```
output/
  master.m3u8
  720p/
    playlist.m3u8
    init.mp4
    segment_0000.m4s
    segment_0001.m4s
    ...
  480p/
    ...
  360p/
    ...
```

### Serve and play

```bash
make serve
# or:
go run cmd/server/main.go -dir ./output -port 8080
```

Open http://localhost:8080 in your browser.

### Build binaries

```bash
make build
./bin/transcode -input video.mp4 -output ./output
./bin/server -dir ./output -port 8080
```

## Encoding Ladder

| Profile | Resolution | Video Bitrate | Audio Bitrate |
|---------|-----------|---------------|---------------|
| 720p    | 1280×720  | 2500k         | 128k          |
| 480p    | 854×480   | 1000k         | 128k          |
| 360p    | 640×360   | 400k          | 96k           |

Profiles whose resolution exceeds the source are automatically excluded.

## Player Features

- Adaptive bitrate playback via hls.js
- Quality overlay: current level, estimated bandwidth, buffer depth, segment load time
- Manual quality selector (Auto + each level)

## MIME Types

| Extension | Content-Type                        |
|-----------|-------------------------------------|
| `.m3u8`   | `application/vnd.apple.mpegurl`     |
| `.m4s`    | `video/iso.bmff`                    |
| `.mp4`    | `video/mp4`                         |
