package video

import (
	"database/sql"

	"philos-video/internal/models"
)

// Repo is the PostgreSQL implementation of Repository.
type Repo struct {
	db *sql.DB
}

// New creates a video Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
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
