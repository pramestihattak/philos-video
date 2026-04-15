package streamkey

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for stream keys.
type Repository interface {
	Create(ctx context.Context, label string, recordVOD bool, userID string) (*models.StreamKey, error)
	GetByID(ctx context.Context, id string) (*models.StreamKey, error)
	List(ctx context.Context, userID string) ([]*models.StreamKey, error)
	Deactivate(ctx context.Context, id, userID string) error
	UpdateRecordVOD(ctx context.Context, id string, recordVOD bool, userID string) error
}
