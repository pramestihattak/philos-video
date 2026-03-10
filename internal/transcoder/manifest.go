package transcoder

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteManifest generates master.m3u8 at the output root.
// Profiles are written highest → lowest bandwidth.
func WriteManifest(outputDir string, profiles []Profile) error {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:7\n")

	for _, p := range profiles {
		bandwidth := parseBandwidth(p.VideoBitrate) + parseBandwidth(p.AudioBitrate)
		fmt.Fprintf(&sb,
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"avc1.64001f,mp4a.40.2\"\n",
			bandwidth, p.Width, p.Height,
		)
		fmt.Fprintf(&sb, "%s/playlist.m3u8\n", p.Name)
	}

	dest := filepath.Join(outputDir, "master.m3u8")
	if err := os.WriteFile(dest, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("writing master.m3u8: %w", err)
	}
	return nil
}

// parseBandwidth converts a bitrate string like "2500k" or "2500000" to bps int.
func parseBandwidth(s string) int {
	s = strings.TrimSpace(s)
	if v, ok := strings.CutSuffix(s, "k"); ok {
		n, _ := strconv.Atoi(v)
		return n * 1000
	}
	if v, ok := strings.CutSuffix(s, "M"); ok {
		n, _ := strconv.Atoi(v)
		return n * 1_000_000
	}
	n, _ := strconv.Atoi(s)
	return n
}
