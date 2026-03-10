package transcoder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Encode transcodes the input file to an intermediate MP4 for the given profile.
func Encode(ctx context.Context, input, outputDir string, p Profile) error {
	profileDir := filepath.Join(outputDir, p.Name)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("creating profile dir: %w", err)
	}

	output := filepath.Join(profileDir, "intermediate.mp4")

	scale := fmt.Sprintf("scale=%d:%d", p.Width, p.Height)

	args := []string{
		"-y",
		"-i", input,
		"-c:v", "libx264",
		"-preset", "medium",
		"-b:v", p.VideoBitrate,
		"-maxrate", p.MaxRate,
		"-bufsize", p.BufSize,
		"-vf", scale,
		"-g", "120",
		"-keyint_min", "120",
		"-sc_threshold", "0",
		"-bf", "2",
		"-c:a", "aac",
		"-b:a", p.AudioBitrate,
		"-ar", "48000",
		"-movflags", "+faststart",
		output,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ffmpeg: %w", err)
	}

	var stderrBuf strings.Builder
	go streamProgress(p.Name, stderr, &stderrBuf)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg encode [%s] failed: %w\nstderr: %s", p.Name, err, stderrBuf.String())
	}

	slog.Info("encode complete", "profile", p.Name, "output", output)
	return nil
}

// streamProgress reads FFmpeg stderr, printing progress lines and buffering the rest.
func streamProgress(profile string, r io.Reader, buf *strings.Builder) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line + "\n")

		if strings.HasPrefix(line, "frame=") || strings.HasPrefix(line, "fps=") {
			slog.Debug("ffmpeg progress", "profile", profile, "line", line)
		}
	}
}
