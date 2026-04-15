package upload

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"philos-video/internal/models"
)

func (s *Service) assemble(ctx context.Context, uploadID string, totalChunks int) error {
	slog.Info("assembling upload", "upload_id", uploadID)

	video, err := s.videos.GetByID(ctx, uploadID)
	if err != nil || video == nil {
		return fmt.Errorf("video not found: %s", uploadID)
	}

	// Prefer original filename for extension (title may not have one).
	ext := ".mp4"
	sidecarPath := filepath.Join(s.dataDir, "chunks", uploadID, ".original_filename")
	if raw, err := os.ReadFile(sidecarPath); err == nil && len(raw) > 0 {
		if e := filepath.Ext(string(raw)); e != "" {
			ext = e
		}
	} else if e := filepath.Ext(video.Title); e != "" {
		ext = e
	}

	rawDir := filepath.Join(s.dataDir, "raw", uploadID)
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return fmt.Errorf("creating raw dir: %w", err)
	}

	outPath := filepath.Join(rawDir, "original"+ext)
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	chunkDir := filepath.Join(s.dataDir, "chunks", uploadID)
	for i := range totalChunks {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%05d", i))
		chunk, err := os.Open(chunkPath)
		if err != nil {
			out.Close()
			return fmt.Errorf("opening chunk %d: %w", i, err)
		}
		if _, err := io.Copy(out, chunk); err != nil {
			chunk.Close()
			out.Close()
			return fmt.Errorf("copying chunk %d: %w", i, err)
		}
		chunk.Close()
	}
	out.Close()

	// Record assembled file size and update user quota usage.
	if fi, err := os.Stat(outPath); err == nil {
		size := fi.Size()
		if err := s.videos.UpdateSizeBytes(ctx, uploadID, size); err != nil {
			slog.Warn("updating video size_bytes", "upload_id", uploadID, "err", err)
		}
		if video.UserID != "" {
			if err := s.userRepo.IncUsedBytes(ctx, video.UserID, size); err != nil {
				slog.Warn("incrementing user used_bytes", "user_id", video.UserID, "err", err)
			}
		}
	}

	if err := os.RemoveAll(chunkDir); err != nil {
		slog.Warn("removing chunk dir", "path", chunkDir, "err", err)
	}

	jobID, err := generateID()
	if err != nil {
		return fmt.Errorf("generating job ID: %w", err)
	}

	job := &models.TranscodeJob{
		ID:      jobID,
		VideoID: uploadID,
		Status:  models.JobStatusQueued,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return fmt.Errorf("creating job: %w", err)
	}

	if err := s.videos.UpdateStatus(ctx, uploadID, models.VideoStatusProcessing); err != nil {
		return fmt.Errorf("updating video status: %w", err)
	}

	slog.Info("assembly complete, job queued", "upload_id", uploadID, "job_id", jobID)
	s.jobCh <- jobID
	return nil
}
