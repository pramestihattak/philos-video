package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"philos-video/internal/transcoder"
)

func main() {
	input := flag.String("input", "", "Path to the input video file (required)")
	output := flag.String("output", "./output", "Directory for HLS output")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "error: -input is required")
		flag.Usage()
		os.Exit(1)
	}

	checkDeps()

	ctx := context.Background()

	slog.Info("probing input", "file", *input)
	info, err := transcoder.Probe(ctx, *input)
	if err != nil {
		slog.Error("probe failed", "err", err)
		os.Exit(1)
	}

	profiles := transcoder.BuildLadder(info.Width, info.Height)
	if len(profiles) == 0 {
		slog.Error("no profiles match source resolution", "width", info.Width, "height", info.Height)
		os.Exit(1)
	}
	for _, p := range profiles {
		slog.Info("will encode", "profile", p.Name, "resolution", fmt.Sprintf("%dx%d", p.Width, p.Height))
	}

	if err := os.MkdirAll(*output, 0o755); err != nil {
		slog.Error("creating output dir", "err", err)
		os.Exit(1)
	}

	for _, p := range profiles {
		slog.Info("encoding", "profile", p.Name)
		if err := transcoder.Encode(ctx, *input, *output, p); err != nil {
			slog.Error("encode failed", "profile", p.Name, "err", err)
			os.Exit(1)
		}

		slog.Info("segmenting", "profile", p.Name)
		if err := transcoder.Segment(ctx, *output, p); err != nil {
			slog.Error("segment failed", "profile", p.Name, "err", err)
			os.Exit(1)
		}
	}

	if err := transcoder.WriteManifest(*output, profiles); err != nil {
		slog.Error("writing manifest", "err", err)
		os.Exit(1)
	}

	slog.Info("done", "output", *output)
}

func checkDeps() {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			fmt.Fprintf(os.Stderr,
				"error: %s not found in PATH.\nInstall FFmpeg: https://ffmpeg.org/download.html\n",
				tool,
			)
			os.Exit(1)
		}
	}
}
