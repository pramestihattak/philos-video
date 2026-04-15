package user

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for users.
type Repository interface {
	UpsertFromGoogle(ctx context.Context, googleSub, email, name, picture string, defaultQuota int64) (*models.User, bool, error)
	GetByID(ctx context.Context, id string) (*models.User, error)
	HasQuotaFor(ctx context.Context, userID string, size int64) (bool, error)
	IncUsedBytes(ctx context.Context, userID string, delta int64) error
	DecUsedBytes(ctx context.Context, userID string, delta int64) error
}
