package video

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// DeleteVideo removes a video and its HLS files. Requires the owning userID.
func (s *Service) DeleteVideo(ctx context.Context, id, userID string) error {
	v, err := s.videos.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("looking up video: %w", err)
	}
	if v == nil {
		return nil // not found or not owner — silent no-op
	}

	if err := s.videos.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting from database: %w", err)
	}

	if v.SizeBytes > 0 && v.UserID != "" {
		if err := s.userRepo.DecUsedBytes(ctx, v.UserID, v.SizeBytes); err != nil {
			slog.Warn("decrementing user used_bytes", "user_id", v.UserID, "err", err)
		}
	}

	hlsDir := filepath.Join(s.dataDir, "hls", id)
	if err := os.RemoveAll(hlsDir); err != nil {
		slog.Warn("removing HLS dir after delete", "path", hlsDir, "err", err)
	}

	slog.Info("video deleted", "video_id", id, "user_id", userID)
	return nil
}
