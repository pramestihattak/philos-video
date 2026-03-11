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

// RemuxClean copies only the first video and first audio stream into a new MP4,
// stripping proprietary/unknown tracks (Apple apac spatial audio, Dolby Vision RPU,
// mebx metadata, etc.). FFmpeg allocates a codec context for every stream in the
// input — even ones excluded by -map — so a file with 9 streams (common on iPhone)
// triggers 9 initializations. On a memory-constrained server this pushes the process
// over the OOM limit before a single frame is encoded.
func RemuxClean(ctx context.Context, input, output string) error {
	args := []string{
		"-y",
		"-i", input,
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c", "copy",
		"-movflags", "+faststart",
		output,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg remux failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

// Encode transcodes the input file to an intermediate MP4 for the given profile.
func Encode(ctx context.Context, input, outputDir string, p Profile) error {
	profileDir := filepath.Join(outputDir, p.Name)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("creating profile dir: %w", err)
	}

	output := filepath.Join(profileDir, "intermediate.mp4")

	// Filter chain (order matters for memory):
	//  1. format=yuv420p  – convert 10-bit sources to 8-bit immediately after the
	//                       decoder so every downstream filter works on smaller frames.
	//  2. fps=fps=30      – cap at 30 fps before scale. High-framerate sources (e.g.
	//                       120 fps iPhone HEVC) fill libx264's rc_lookahead buffer
	//                       4× faster, consuming ~250 MB before a single frame is
	//                       output. 30 fps is the web-streaming standard anyway.
	//  3. scale            – resize after the two cheap conversions above.
	vf := fmt.Sprintf(
		"format=yuv420p,fps=fps=30,scale='if(gt(iw,ih),-2,min(%d,iw))':'if(gt(iw,ih),min(%d,ih),-2)'",
		p.Height, p.Height,
	)

	args := []string{
		"-y",
		"-i", input,
		// Explicitly map only the first video and first audio stream.
		// Skips unknown/proprietary tracks (e.g. Apple 'apac' spatial audio, Dolby
		// Vision RPU, metadata tracks) that would cause FFmpeg to fail or be killed.
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "libx264",
		"-preset", "fast",
		// rc_lookahead: fast=20 frames vs medium=40. For 4K source decoded to 1080p,
		// 40 frames × ~3 MB = ~120 MB lookahead alone; fast halves this safely.
		"-b:v", p.VideoBitrate,
		"-maxrate", p.MaxRate,
		"-bufsize", p.BufSize,
		"-vf", vf,
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
