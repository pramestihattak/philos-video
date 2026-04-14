package worker

import (
	"context"
	"log/slog"
	"sync"

	"philos-video/internal/metrics"
	"philos-video/internal/models"
	"philos-video/internal/service"
	"philos-video/internal/storage"
)

type TranscodeWorker struct {
	jobs      storage.JobStorer
	videos    storage.VideoStorer
	transcode *service.TranscodeService
	jobCh     <-chan string
	wg        sync.WaitGroup
}

func NewTranscodeWorker(
	jobs storage.JobStorer,
	videos storage.VideoStorer,
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
		w.wg.Add(1)
		go w.run(ctx, i)
	}
}

// Wait blocks until all worker goroutines have finished.
func (w *TranscodeWorker) Wait() {
	w.wg.Wait()
}

func (w *TranscodeWorker) run(ctx context.Context, workerID int) {
	defer w.wg.Done()
	slog.Info("worker started", "worker_id", workerID)
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopping", "worker_id", workerID)
			return
		case jobID, ok := <-w.jobCh:
			if !ok {
				slog.Info("worker channel closed", "worker_id", workerID)
				return
			}
			slog.Info("processing job", "worker_id", workerID, "job_id", jobID)
			metrics.TranscodeActiveWorkers.Inc()
			metrics.TranscodeJobsTotal.WithLabelValues("started").Inc()

			if err := w.transcode.Process(ctx, jobID); err != nil {
				slog.Error("job failed", "worker_id", workerID, "job_id", jobID, "err", err)
				metrics.TranscodeJobsTotal.WithLabelValues("failed").Inc()
				_ = w.jobs.Fail(ctx, jobID, err.Error())
				if job, _ := w.jobs.GetByID(ctx, jobID); job != nil {
					_ = w.videos.UpdateStatus(ctx, job.VideoID, models.VideoStatusFailed)
				}
			} else {
				metrics.TranscodeJobsTotal.WithLabelValues("completed").Inc()
			}
			metrics.TranscodeActiveWorkers.Dec()
		}
	}
}
