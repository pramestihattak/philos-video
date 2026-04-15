package upload

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"philos-video/internal/models"
)

// InitUpload creates a video record and chunk slots. It enforces the per-user
// quota using the reported expectedSize (bytes). Pass 0 to skip the quota
// check (e.g. for chunked uploads where size is unknown upfront).
// title defaults to filename when empty. visibility defaults to "public" when empty or invalid.
func (s *Service) InitUpload(ctx context.Context, user *models.User, filename, title, visibility string, totalChunks int, expectedSize int64) (string, error) {
	if expectedSize > 0 {
		ok, err := s.userRepo.HasQuotaFor(ctx, user.ID, expectedSize)
		if err != nil {
			return "", fmt.Errorf("checking quota: %w", err)
		}
		if !ok {
			return "", ErrQuotaExceeded
		}
	}

	if title == "" {
		title = filename
	}
	switch visibility {
	case models.VisibilityPrivate, models.VisibilityUnlisted, models.VisibilityPublic:
	default:
		visibility = models.VisibilityPublic
	}

	id, err := generateID()
	if err != nil {
		return "", fmt.Errorf("generating ID: %w", err)
	}

	video := &models.Video{
		ID:         id,
		UserID:     user.ID,
		Title:      title,
		Visibility: visibility,
		Status:     models.VideoStatusUploading,
	}
	if err := s.videos.Create(ctx, video); err != nil {
		return "", fmt.Errorf("creating video record: %w", err)
	}

	if err := s.uploads.CreateChunks(ctx, id, totalChunks); err != nil {
		return "", fmt.Errorf("creating chunk records: %w", err)
	}

	chunkDir := filepath.Join(s.dataDir, "chunks", id)
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return "", fmt.Errorf("creating chunk dir: %w", err)
	}

	// Write the original filename to a sidecar file so assemble() can infer the extension
	// without depending on the display title (which may not have an extension).
	sidecarPath := filepath.Join(s.dataDir, "chunks", id, ".original_filename")
	if err := os.MkdirAll(filepath.Join(s.dataDir, "chunks", id), 0o755); err == nil {
		_ = os.WriteFile(sidecarPath, []byte(filename), 0o600)
	}

	slog.Info("upload initialized", "upload_id", id, "filename", filename, "total_chunks", totalChunks, "user_id", user.ID)
	return id, nil
}
