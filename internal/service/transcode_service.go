package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"philos-video/internal/models"
	"philos-video/internal/repository"
	"philos-video/internal/transcoder"
)

type TranscodeService struct {
	videos  *repository.VideoRepo
	jobs    *repository.JobRepo
	dataDir string
}

func NewTranscodeService(
	videos *repository.VideoRepo,
	jobs *repository.JobRepo,
	dataDir string,
) *TranscodeService {
	return &TranscodeService{videos: videos, jobs: jobs, dataDir: dataDir}
}

func (s *TranscodeService) Process(ctx context.Context, jobID string) error {
	job, err := s.jobs.GetByID(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if err := s.jobs.UpdateRunning(jobID); err != nil {
		return fmt.Errorf("marking job running: %w", err)
	}

	video, err := s.videos.GetByID(job.VideoID)
	if err != nil || video == nil {
		return fmt.Errorf("video not found: %s", job.VideoID)
	}

	rawDir := filepath.Join(s.dataDir, "raw", video.ID)
	hlsDir := filepath.Join(s.dataDir, "hls", video.ID)

	// Find input file
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return fmt.Errorf("reading raw dir: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("no input file in %s", rawDir)
	}
	inputPath := filepath.Join(rawDir, entries[0].Name())

	// probe (0.05)
	_ = s.jobs.UpdateProgress(jobID, "probe", 0.05)
	slog.Info("probing", "job_id", jobID, "input", inputPath)

	info, err := transcoder.Probe(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("probe failed: %w", err)
	}
	_ = s.videos.UpdateAfterProbe(video.ID, info.Width, info.Height, info.Duration, info.Codec)

	// prepare (0.10)
	_ = s.jobs.UpdateProgress(jobID, "prepare", 0.10)
	profiles := transcoder.BuildLadder(info.Width, info.Height)
	if len(profiles) == 0 {
		return fmt.Errorf("no suitable profiles for %dx%d", info.Width, info.Height)
	}

	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		return fmt.Errorf("creating hls dir: %w", err)
	}

	// encode + segment per profile (0.10–0.80)
	n := float64(len(profiles))
	for i, p := range profiles {
		encProg := 0.10 + float64(i)/n*0.70
		_ = s.jobs.UpdateProgress(jobID, "encode:"+p.Name, encProg)
		slog.Info("encoding", "job_id", jobID, "profile", p.Name)

		if err := transcoder.Encode(ctx, inputPath, hlsDir, p); err != nil {
			return fmt.Errorf("encode [%s]: %w", p.Name, err)
		}

		segProg := 0.10 + (float64(i)+0.5)/n*0.70
		_ = s.jobs.UpdateProgress(jobID, "segment:"+p.Name, segProg)
		slog.Info("segmenting", "job_id", jobID, "profile", p.Name)

		if err := transcoder.Segment(ctx, hlsDir, p); err != nil {
			return fmt.Errorf("segment [%s]: %w", p.Name, err)
		}
	}

	// packaging (0.95)
	_ = s.jobs.UpdateProgress(jobID, "packaging", 0.95)
	slog.Info("writing manifest", "job_id", jobID)

	if err := transcoder.WriteManifest(hlsDir, profiles); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	_ = os.RemoveAll(rawDir)

	hlsPath := filepath.Join("hls", video.ID)
	_ = s.videos.UpdateHLSPath(video.ID, hlsPath)

	if err := s.videos.UpdateStatus(video.ID, models.VideoStatusReady); err != nil {
		return fmt.Errorf("updating video status: %w", err)
	}
	if err := s.jobs.Complete(jobID); err != nil {
		return fmt.Errorf("completing job: %w", err)
	}

	slog.Info("transcode complete", "job_id", jobID, "video_id", video.ID)
	return nil
}
