package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"philos-video/internal/models"
	"philos-video/internal/repository"
)

type UploadService struct {
	videos  *repository.VideoRepo
	uploads *repository.UploadRepo
	jobs    *repository.JobRepo
	dataDir string
	jobCh   chan<- string
}

func NewUploadService(
	videos *repository.VideoRepo,
	uploads *repository.UploadRepo,
	jobs *repository.JobRepo,
	dataDir string,
	jobCh chan<- string,
) *UploadService {
	return &UploadService{
		videos:  videos,
		uploads: uploads,
		jobs:    jobs,
		dataDir: dataDir,
		jobCh:   jobCh,
	}
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *UploadService) InitUpload(ctx context.Context, filename string, totalChunks int) (string, error) {
	id, err := generateID()
	if err != nil {
		return "", fmt.Errorf("generating ID: %w", err)
	}

	video := &models.Video{
		ID:     id,
		Title:  filename,
		Status: models.VideoStatusUploading,
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

	slog.Info("upload initialized", "upload_id", id, "filename", filename, "total_chunks", totalChunks)
	return id, nil
}

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
			if err := s.assemble(context.Background(), uploadID, total); err != nil {
				slog.Error("assembly failed", "upload_id", uploadID, "err", err)
				_ = s.videos.UpdateStatus(uploadID, models.VideoStatusFailed)
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

	ext := filepath.Ext(video.Title)
	if ext == "" {
		ext = ".mp4"
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

	_ = os.RemoveAll(chunkDir)

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
