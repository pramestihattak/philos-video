package worker

import (
	"context"
	"log/slog"

	"philos-video/internal/models"
	"philos-video/internal/repository"
	"philos-video/internal/service"
)

type TranscodeWorker struct {
	jobs      *repository.JobRepo
	videos    *repository.VideoRepo
	transcode *service.TranscodeService
	jobCh     <-chan string
}

func NewTranscodeWorker(
	jobs *repository.JobRepo,
	videos *repository.VideoRepo,
	transcode *service.TranscodeService,
	jobCh <-chan string,
) *TranscodeWorker {
	return &TranscodeWorker{
		jobs:      jobs,
		videos:    videos,
		transcode: transcode,
		jobCh:     jobCh,
	}
}

func (w *TranscodeWorker) Start(ctx context.Context, n int) {
	for i := range n {
		go w.run(ctx, i)
	}
}

func (w *TranscodeWorker) run(ctx context.Context, workerID int) {
	slog.Info("worker started", "worker_id", workerID)
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopping", "worker_id", workerID)
			return
		case jobID, ok := <-w.jobCh:
			if !ok {
				return
			}
			slog.Info("processing job", "worker_id", workerID, "job_id", jobID)
			if err := w.transcode.Process(ctx, jobID); err != nil {
				slog.Error("job failed", "worker_id", workerID, "job_id", jobID, "err", err)
				_ = w.jobs.Fail(jobID, err.Error())
				if job, _ := w.jobs.GetByID(jobID); job != nil {
					_ = w.videos.UpdateStatus(job.VideoID, models.VideoStatusFailed)
				}
			}
		}
	}
}
