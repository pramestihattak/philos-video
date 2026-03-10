package transcoder

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// Segment converts an intermediate MP4 into fMP4 HLS segments.
func Segment(ctx context.Context, outputDir string, p Profile) error {
	profileDir := filepath.Join(outputDir, p.Name)
	intermediate := filepath.Join(profileDir, "intermediate.mp4")
	playlist := filepath.Join(profileDir, "playlist.m3u8")
	segmentPattern := filepath.Join(profileDir, "segment_%04d.m4s")

	args := []string{
		"-y",
		"-i", intermediate,
		"-c", "copy",
		"-f", "hls",
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_segment_filename", segmentPattern,
		"-hls_fmp4_init_filename", "init.mp4",
		playlist,
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg segment [%s] failed: %w\nstderr: %s", p.Name, err, stderr.String())
	}

	// Remove intermediate file after successful segmentation.
	if err := os.Remove(intermediate); err != nil {
		slog.Warn("could not remove intermediate file", "path", intermediate, "err", err)
	}

	slog.Info("segmentation complete", "profile", p.Name, "playlist", playlist)
	return nil
}
