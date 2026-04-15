package upload

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"philos-video/internal/metrics"
	"philos-video/internal/models"
)

func (s *Service) ReceiveChunk(ctx context.Context, uploadID string, chunkNumber int, data io.Reader) error {
	chunkPath := filepath.Join(s.dataDir, "chunks", uploadID, fmt.Sprintf("%05d", chunkNumber))
	f, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("creating chunk file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("writing chunk: %w", err)
	}
	f.Close()

	if err := s.uploads.MarkChunkReceived(ctx, uploadID, chunkNumber); err != nil {
		return fmt.Errorf("marking chunk received: %w", err)
	}

	received, total, err := s.uploads.GetProgress(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("getting progress: %w", err)
	}

	slog.Info("chunk received", "upload_id", uploadID, "chunk", chunkNumber,
		"progress", fmt.Sprintf("%d/%d", received, total))

	if received == total {
		go func() {
			assembleCtx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()
			if err := s.assemble(assembleCtx, uploadID, total); err != nil {
				slog.Error("assembly failed", "upload_id", uploadID, "err", err)
				metrics.UploadsTotal.WithLabelValues("failed").Inc()
				metrics.ActiveUploads.Dec()
				_ = s.videos.UpdateStatus(assembleCtx, uploadID, models.VideoStatusFailed)
			} else {
				metrics.UploadsTotal.WithLabelValues("completed").Inc()
				metrics.ActiveUploads.Dec()
			}
		}()
	}

	return nil
}
