package live

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const (
	flvTagVideo = 0x09
	flvTagAudio = 0x08
)

// flvFileHeader is the 13-byte FLV file signature (9 bytes) + PreviousTagSize0 (4 bytes).
var flvFileHeader = []byte{
	'F', 'L', 'V',         // signature
	0x01,                   // version 1
	0x05,                   // type flags: audio (bit 2) + video (bit 0)
	0x00, 0x00, 0x00, 0x09, // data offset = 9
	0x00, 0x00, 0x00, 0x00, // PreviousTagSize0 = 0
}

// liveMasterPlaylist is the pre-built HLS master playlist for live streams.
const liveMasterPlaylist = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=2628000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720p/playlist.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1096000,RESOLUTION=854x480,CODECS="avc1.64001f,mp4a.40.2"
480p/playlist.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=464000,RESOLUTION=640x360,CODECS="avc1.64001f,mp4a.40.2"
360p/playlist.m3u8
`

type transcodeSession struct {
	streamID  string
	outputDir string
	cmd       *exec.Cmd
	stdin     io.WriteCloser

	mu     sync.Mutex
	closed bool
}

func newTranscodeSession(streamID, dataDir string) (*transcodeSession, error) {
	outputDir := filepath.Join(dataDir, "live", streamID)

	// Create quality subdirectories.
	for _, d := range []string{"", "720p", "480p", "360p"} {
		if err := os.MkdirAll(filepath.Join(outputDir, d), 0o755); err != nil {
			return nil, fmt.Errorf("creating live dir: %w", err)
		}
	}

	// Write master playlist upfront — we know the profiles.
	masterPath := filepath.Join(outputDir, "master.m3u8")
	if err := os.WriteFile(masterPath, []byte(liveMasterPlaylist), 0o644); err != nil {
		return nil, fmt.Errorf("writing master playlist: %w", err)
	}

	// Build FFmpeg arguments for multi-quality live HLS.
	filterComplex := "[0:v]split=3[raw720][raw480][raw360];" +
		"[raw720]scale=1280:720[v720];" +
		"[raw480]scale=854:480[v480];" +
		"[raw360]scale=640:360[v360]"

	segPattern := filepath.Join(outputDir, "%v", "segment_%04d.ts")
	outPattern := filepath.Join(outputDir, "%v", "playlist.m3u8")

	args := []string{
		"-f", "flv", "-i", "pipe:0",
		"-filter_complex", filterComplex,
		// Map video and audio streams.
		"-map", "[v720]", "-map", "[v480]", "-map", "[v360]",
		"-map", "0:a", "-map", "0:a", "-map", "0:a",
		// Video codec settings per stream.
		"-c:v:0", "libx264", "-preset:v:0", "veryfast", "-tune:v:0", "zerolatency",
		"-b:v:0", "2500k", "-maxrate:v:0", "2500k", "-bufsize:v:0", "5000k", "-g:v:0", "60",
		"-c:v:1", "libx264", "-preset:v:1", "veryfast", "-tune:v:1", "zerolatency",
		"-b:v:1", "1000k", "-maxrate:v:1", "1000k", "-bufsize:v:1", "2000k", "-g:v:1", "60",
		"-c:v:2", "libx264", "-preset:v:2", "veryfast", "-tune:v:2", "zerolatency",
		"-b:v:2", "400k", "-maxrate:v:2", "400k", "-bufsize:v:2", "800k", "-g:v:2", "60",
		// Audio codec settings per stream.
		"-c:a:0", "aac", "-b:a:0", "128k", "-ar:a:0", "48000",
		"-c:a:1", "aac", "-b:a:1", "96k", "-ar:a:1", "48000",
		"-c:a:2", "aac", "-b:a:2", "64k", "-ar:a:2", "48000",
		// HLS output — no master playlist (we wrote it ourselves).
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+independent_segments+append_list",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", segPattern,
		"-var_stream_map", "v:0,a:0,name:720p v:1,a:1,name:480p v:2,a:2,name:360p",
		outPattern,
	}

	cmd := exec.Command("ffmpeg", args...)

	// Feed FLV data via stdin pipe.
	pr, pw := io.Pipe()
	cmd.Stdin = pr
	cmd.Stderr = os.Stderr // surface FFmpeg logs to server stderr

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	ts := &transcodeSession{
		streamID:  streamID,
		outputDir: outputDir,
		cmd:       cmd,
		stdin:     pw,
	}

	// Write FLV file header so FFmpeg sees a valid FLV stream.
	if _, err := pw.Write(flvFileHeader); err != nil {
		ts.stop()
		return nil, fmt.Errorf("writing FLV header: %w", err)
	}

	return ts, nil
}

func (ts *transcodeSession) writeTag(tagType byte, timestamp uint32, payload []byte) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return nil
	}

	dataSize := uint32(len(payload))
	prevTagSize := dataSize + 11

	var buf bytes.Buffer
	buf.Grow(11 + len(payload) + 4)

	buf.WriteByte(tagType)
	// Data size: 3 bytes big-endian
	buf.WriteByte(byte(dataSize >> 16))
	buf.WriteByte(byte(dataSize >> 8))
	buf.WriteByte(byte(dataSize))
	// Timestamp: lower 24 bits
	buf.WriteByte(byte(timestamp >> 16))
	buf.WriteByte(byte(timestamp >> 8))
	buf.WriteByte(byte(timestamp))
	// TimestampExtended: upper 8 bits
	buf.WriteByte(byte(timestamp >> 24))
	// Stream ID: always 0
	buf.WriteByte(0)
	buf.WriteByte(0)
	buf.WriteByte(0)
	// Tag data
	buf.Write(payload)
	// PreviousTagSize: 4 bytes big-endian
	buf.WriteByte(byte(prevTagSize >> 24))
	buf.WriteByte(byte(prevTagSize >> 16))
	buf.WriteByte(byte(prevTagSize >> 8))
	buf.WriteByte(byte(prevTagSize))

	_, err := ts.stdin.Write(buf.Bytes())
	return err
}

func (ts *transcodeSession) writeVideo(timestamp uint32, payload io.Reader) error {
	data, err := io.ReadAll(payload)
	if err != nil {
		return err
	}
	return ts.writeTag(flvTagVideo, timestamp, data)
}

func (ts *transcodeSession) writeAudio(timestamp uint32, payload io.Reader) error {
	data, err := io.ReadAll(payload)
	if err != nil {
		return err
	}
	return ts.writeTag(flvTagAudio, timestamp, data)
}

func (ts *transcodeSession) stop() {
	ts.mu.Lock()
	if ts.closed {
		ts.mu.Unlock()
		return
	}
	ts.closed = true
	ts.stdin.Close()
	ts.mu.Unlock()

	if err := ts.cmd.Wait(); err != nil {
		slog.Warn("ffmpeg live exited", "stream_id", ts.streamID, "err", err)
	}

	// Finalise variant playlists with ENDLIST so players know the stream ended.
	for _, q := range []string{"720p", "480p", "360p"} {
		plPath := filepath.Join(ts.outputDir, q, "playlist.m3u8")
		f, err := os.OpenFile(plPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Warn("opening playlist for ENDLIST", "path", plPath, "err", err)
			continue
		}
		_, _ = f.WriteString("#EXT-X-ENDLIST\n")
		f.Close()
	}
}
