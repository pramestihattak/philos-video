package repository

import (
	"database/sql"
	"time"

	"philos-video/internal/models"
)

type VideoRepo struct {
	db *sql.DB
}

func NewVideoRepo(db *sql.DB) *VideoRepo {
	return &VideoRepo{db: db}
}

func (r *VideoRepo) Create(v *models.Video) error {
	_, err := r.db.Exec(
		`INSERT INTO videos (id, user_id, title, visibility, status) VALUES ($1, $2, $3, $4, $5)`,
		v.ID, v.UserID, v.Title, v.Visibility, v.Status,
	)
	return err
}

// getByIDQuery fetches a single video by ID using a correlated subquery for play count
// (single-row lookup — subquery cost is acceptable here).
const getByIDQuery = `
	SELECT v.id, v.user_id, COALESCE(u.name, u.email, ''), COALESCE(u.picture, ''), v.title, v.visibility, v.status,
	       COALESCE(v.width,0), COALESCE(v.height,0),
	       COALESCE(v.duration,''), COALESCE(v.codec,''), COALESCE(v.hls_path,''),
	       COALESCE(v.size_bytes,0), COALESCE(v.thumbnail_path,''),
	       (SELECT COUNT(*) FROM playback_sessions ps WHERE ps.video_id = v.id),
	       v.created_at, v.updated_at
	FROM videos v
	LEFT JOIN users u ON u.id = v.user_id
	WHERE v.id = $1`

// listQuery returns public ready videos from all users combined with all
// videos belonging to the requesting user (any visibility, any status).
const listQuery = `
	SELECT v.id, v.user_id, COALESCE(u.name, u.email, ''), COALESCE(u.picture, ''), v.title, v.visibility, v.status,
	       COALESCE(v.width,0), COALESCE(v.height,0),
	       COALESCE(v.duration,''), COALESCE(v.codec,''), COALESCE(v.hls_path,''),
	       COALESCE(v.size_bytes,0), COALESCE(v.thumbnail_path,''),
	       COUNT(ps.id),
	       v.created_at, v.updated_at
	FROM videos v
	LEFT JOIN users u ON u.id = v.user_id
	LEFT JOIN playback_sessions ps ON ps.video_id = v.id
	WHERE (v.visibility = 'public' AND v.status = 'ready')
	   OR v.user_id = $3
	GROUP BY v.id, u.name, u.email, u.picture
	ORDER BY v.created_at DESC
	LIMIT $1 OFFSET $2`

// listPublicQuery returns public ready videos visible to guests.
// Unlisted videos are accessible by direct link only — not listed here.
const listPublicQuery = `
	SELECT v.id, v.user_id, COALESCE(u.name, u.email, ''), COALESCE(u.picture, ''), v.title, v.visibility, v.status,
	       COALESCE(v.width,0), COALESCE(v.height,0),
	       COALESCE(v.duration,''), COALESCE(v.codec,''), COALESCE(v.hls_path,''),
	       COALESCE(v.size_bytes,0), COALESCE(v.thumbnail_path,''),
	       COUNT(ps.id),
	       v.created_at, v.updated_at
	FROM videos v
	LEFT JOIN users u ON u.id = v.user_id
	LEFT JOIN playback_sessions ps ON ps.video_id = v.id
	WHERE v.visibility = 'public' AND v.status = 'ready'
	GROUP BY v.id, u.name, u.email, u.picture
	ORDER BY v.created_at DESC
	LIMIT $1 OFFSET $2`

func scanVideo(s interface{ Scan(...any) error }) (*models.Video, error) {
	v := &models.Video{}
	err := s.Scan(
		&v.ID, &v.UserID, &v.UploaderName, &v.UploaderPicture, &v.Title, &v.Visibility, &v.Status,
		&v.Width, &v.Height, &v.Duration, &v.Codec, &v.HLSPath,
		&v.SizeBytes, &v.ThumbnailPath, &v.PlayCount, &v.CreatedAt, &v.UpdatedAt,
	)
	return v, err
}

// GetByID returns a video by ID regardless of owner (used by public playback paths).
func (r *VideoRepo) GetByID(id string) (*models.Video, error) {
	row := r.db.QueryRow(getByIDQuery, id)
	v, err := scanVideo(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

// GetByIDForUser returns a video only if it belongs to the given user.
func (r *VideoRepo) GetByIDForUser(id, userID string) (*models.Video, error) {
	row := r.db.QueryRow(getByIDQuery+` AND v.user_id = $2`, id, userID)
	v, err := scanVideo(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

// ListPublic returns public/unlisted ready videos visible to guests.
func (r *VideoRepo) ListPublic(limit, offset int) ([]*models.Video, error) {
	rows, err := r.db.Query(listPublicQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// List returns up to limit videos for the given user, ordered by creation time descending.
func (r *VideoRepo) List(limit, offset int, userID string) ([]*models.Video, error) {
	rows, err := r.db.Query(listQuery, limit, offset, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

func (r *VideoRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id,
	)
	return err
}

func (r *VideoRepo) UpdateAfterProbe(id string, width, height int, duration, codec string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET width=$1, height=$2, duration=$3, codec=$4, updated_at=$5 WHERE id=$6`,
		width, height, duration, codec, time.Now(), id,
	)
	return err
}

func (r *VideoRepo) UpdateHLSPath(id, hlsPath string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET hls_path=$1, updated_at=$2 WHERE id=$3`,
		hlsPath, time.Now(), id,
	)
	return err
}

func (r *VideoRepo) UpdateSizeBytes(id string, size int64) error {
	_, err := r.db.Exec(
		`UPDATE videos SET size_bytes=$1, updated_at=$2 WHERE id=$3`,
		size, time.Now(), id,
	)
	return err
}

func (r *VideoRepo) UpdateThumbnailPath(id, thumbnailPath string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET thumbnail_path=$1, updated_at=$2 WHERE id=$3`,
		thumbnailPath, time.Now(), id,
	)
	return err
}

// UpdateVisibility changes the visibility of a video, scoped to its owner.
func (r *VideoRepo) UpdateVisibility(id, userID, visibility string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET visibility=$1, updated_at=$2 WHERE id=$3 AND user_id=$4`,
		visibility, time.Now(), id, userID,
	)
	return err
}

// Delete removes a video and all dependent rows in a transaction.
// It requires the owning userID to prevent cross-user deletes.
func (r *VideoRepo) Delete(id, userID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Confirm ownership before touching anything.
	var exists bool
	if err := tx.QueryRow(`SELECT TRUE FROM videos WHERE id=$1 AND user_id=$2`, id, userID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return nil // not found or not owner — treat as no-op
		}
		return err
	}

	// Delete comments for this video.
	if _, err := tx.Exec(`DELETE FROM comments WHERE video_id = $1`, id); err != nil {
		return err
	}

	// Delete events for sessions belonging to this video.
	if _, err := tx.Exec(`
		DELETE FROM playback_events pe
		USING playback_sessions ps
		WHERE pe.session_id = ps.id AND ps.video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM playback_sessions WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM transcode_jobs WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM videos WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit()
}
