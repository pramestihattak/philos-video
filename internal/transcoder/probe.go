package transcoder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
)

// VideoInfo holds the parsed video stream metadata.
type VideoInfo struct {
	Width     int
	Height    int
	Codec     string
	FrameRate string
	Duration  string
	Bitrate   string
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

// Probe runs ffprobe on inputPath and returns video metadata.
func Probe(ctx context.Context, inputPath string) (*VideoInfo, error) {
	args := []string{
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		inputPath,
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w\nstderr: %s", err, stderr.String())
	}

	var out ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parsing ffprobe output: %w", err)
	}

	info := &VideoInfo{
		Duration: out.Format.Duration,
		Bitrate:  out.Format.BitRate,
	}

	for _, s := range out.Streams {
		if s.CodecType == "video" {
			info.Width = s.Width
			info.Height = s.Height
			info.Codec = s.CodecName
			info.FrameRate = s.RFrameRate
			break
		}
	}

	if info.Width == 0 || info.Height == 0 {
		return nil, fmt.Errorf("no video stream found in %s", inputPath)
	}

	bitrateMbps := ""
	if br, err := strconv.ParseFloat(info.Bitrate, 64); err == nil {
		bitrateMbps = fmt.Sprintf("%.2f Mbps", br/1_000_000)
	}

	slog.Info("probe summary",
		"file", inputPath,
		"resolution", fmt.Sprintf("%dx%d", info.Width, info.Height),
		"codec", info.Codec,
		"frame_rate", info.FrameRate,
		"duration", info.Duration+"s",
		"bitrate", bitrateMbps,
	)

	return info, nil
}
