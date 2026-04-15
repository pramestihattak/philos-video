package upload

import (
	"context"
	"io"

	"philos-video/internal/models"
)

// Servicer is the interface for upload management operations.
type Servicer interface {
	InitUpload(ctx context.Context, user *models.User, filename, title, visibility string, totalChunks int, expectedSize int64) (string, error)
	ReceiveChunk(ctx context.Context, uploadID string, chunkNumber int, data io.Reader) error
	GetProgress(ctx context.Context, uploadID string) (received, total int, err error)
}
