package upload

import "context"

// Repository defines the persistence operations for chunked uploads.
type Repository interface {
	CreateChunks(ctx context.Context, uploadID string, totalChunks int) error
	MarkChunkReceived(ctx context.Context, uploadID string, chunkNumber int) error
	GetProgress(ctx context.Context, uploadID string) (received, total int, err error)
}
