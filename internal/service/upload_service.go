package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"philos-video/internal/metrics"
	"philos-video/internal/models"
	"philos-video/internal/repository"
)

type UploadService struct {
	videos   *repository.VideoRepo
	uploads  *repository.UploadRepo
	jobs     *repository.JobRepo
	userRepo *repository.UserRepo
	dataDir  string
	jobCh    chan<- string
}

func NewUploadService(
	videos *repository.VideoRepo,
	uploads *repository.UploadRepo,
	jobs *repository.JobRepo,
	userRepo *repository.UserRepo,
	dataDir string,
	jobCh chan<- string,
) *UploadService {
	return &UploadService{
		videos:   videos,
		uploads:  uploads,
		jobs:     jobs,
		userRepo: userRepo,
		dataDir:  dataDir,
		jobCh:    jobCh,
	}
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// InitUpload creates a video record and chunk slots. It enforces the per-user
// quota using the reported expectedSize (bytes). Pass 0 to skip the quota
// check (e.g. for chunked uploads where size is unknown upfront).
// title defaults to filename when empty. visibility defaults to "public" when empty or invalid.
func (s *UploadService) InitUpload(ctx context.Context, user *models.User, filename, title, visibility string, totalChunks int, expectedSize int64) (string, error) {
	// Quota check.
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
	if err := s.videos.Create(video); err != nil {
		return "", fmt.Errorf("creating video record: %w", err)
	}

	if err := s.uploads.CreateChunks(id, totalChunks); err != nil {
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

// ErrQuotaExceeded is returned when a user's upload quota would be exceeded.
var ErrQuotaExceeded = &quotaError{}

type quotaError struct{}

func (e *quotaError) Error() string { return "upload quota exceeded" }
func (e *quotaError) HTTPStatus() int { return http.StatusRequestEntityTooLarge }

func (s *UploadService) ReceiveChunk(ctx context.Context, uploadID string, chunkNumber int, data io.Reader) error {
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

	if err := s.uploads.MarkChunkReceived(uploadID, chunkNumber); err != nil {
		return fmt.Errorf("marking chunk received: %w", err)
	}

	received, total, err := s.uploads.GetProgress(uploadID)
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
				_ = s.videos.UpdateStatus(uploadID, models.VideoStatusFailed)
			} else {
				metrics.UploadsTotal.WithLabelValues("completed").Inc()
				metrics.ActiveUploads.Dec()
			}
		}()
	}

	return nil
}

func (s *UploadService) assemble(ctx context.Context, uploadID string, totalChunks int) error {
	slog.Info("assembling upload", "upload_id", uploadID)

	video, err := s.videos.GetByID(uploadID)
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
		if err := s.videos.UpdateSizeBytes(uploadID, size); err != nil {
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
	if err := s.jobs.Create(job); err != nil {
		return fmt.Errorf("creating job: %w", err)
	}

	if err := s.videos.UpdateStatus(uploadID, models.VideoStatusProcessing); err != nil {
		return fmt.Errorf("updating video status: %w", err)
	}

	slog.Info("assembly complete, job queued", "upload_id", uploadID, "job_id", jobID)
	s.jobCh <- jobID
	return nil
}

func (s *UploadService) GetProgress(uploadID string) (received, total int, err error) {
	return s.uploads.GetProgress(uploadID)
}
