package video

import (
	"context"
	"time"
)

func (r *Repo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE videos SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateAfterProbe(ctx context.Context, id string, width, height int, duration, codec string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE videos SET width=$1, height=$2, duration=$3, codec=$4, updated_at=$5 WHERE id=$6`,
		width, height, duration, codec, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateHLSPath(ctx context.Context, id, hlsPath string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE videos SET hls_path=$1, updated_at=$2 WHERE id=$3`,
		hlsPath, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateSizeBytes(ctx context.Context, id string, size int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE videos SET size_bytes=$1, updated_at=$2 WHERE id=$3`,
		size, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateThumbnailPath(ctx context.Context, id, thumbnailPath string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE videos SET thumbnail_path=$1, updated_at=$2 WHERE id=$3`,
		thumbnailPath, time.Now(), id,
	)
	return err
}

// UpdateVisibility changes the visibility of a video, scoped to its owner.
func (r *Repo) UpdateVisibility(ctx context.Context, id, userID, visibility string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE videos SET visibility=$1, updated_at=$2 WHERE id=$3 AND user_id=$4`,
		visibility, time.Now(), id, userID,
	)
	return err
}
