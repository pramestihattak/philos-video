package livestream

import (
	"database/sql"

	"philos-video/internal/models"
)

// Repo is the PostgreSQL implementation of Repository.
type Repo struct {
	db *sql.DB
}

// New creates a live stream Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const cols = `id, user_id, stream_key_id, title, status, record_vod,
    source_width, source_height, source_codec, source_fps,
    hls_path, video_id, started_at, ended_at, created_at`

func scanLiveStream(row interface{ Scan(...any) error }) (*models.LiveStream, error) {
	ls := &models.LiveStream{}
	var sourceWidth, sourceHeight sql.NullInt64
	var sourceCodec, sourceFPS, hlsPath, videoID sql.NullString
	err := row.Scan(
		&ls.ID, &ls.UserID, &ls.StreamKeyID, &ls.Title, &ls.Status, &ls.RecordVOD,
		&sourceWidth, &sourceHeight, &sourceCodec, &sourceFPS,
		&hlsPath, &videoID, &ls.StartedAt, &ls.EndedAt, &ls.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if sourceWidth.Valid {
		ls.SourceWidth = int(sourceWidth.Int64)
	}
	if sourceHeight.Valid {
		ls.SourceHeight = int(sourceHeight.Int64)
	}
	if sourceCodec.Valid {
		ls.SourceCodec = sourceCodec.String
	}
	if sourceFPS.Valid {
		ls.SourceFPS = sourceFPS.String
	}
	if hlsPath.Valid {
		ls.HLSPath = hlsPath.String
	}
	if videoID.Valid {
		ls.VideoID = videoID.String
	}
	return ls, nil
}
