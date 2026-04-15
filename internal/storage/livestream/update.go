package livestream

import "context"

func (r *Repo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE live_streams SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *Repo) UpdateStarted(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE live_streams SET status = 'live', started_at = NOW() WHERE id = $1`, id,
	)
	return err
}

func (r *Repo) UpdateEnded(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE live_streams SET status = 'ended', ended_at = NOW() WHERE id = $1`, id,
	)
	return err
}

func (r *Repo) UpdateHLSPath(ctx context.Context, id, hlsPath string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE live_streams SET hls_path = $1 WHERE id = $2`, hlsPath, id)
	return err
}

func (r *Repo) UpdateVideoID(ctx context.Context, id, videoID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE live_streams SET video_id = $1 WHERE id = $2`, videoID, id)
	return err
}

func (r *Repo) UpdateSourceInfo(ctx context.Context, id string, width, height int, codec, fps string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE live_streams SET source_width=$1, source_height=$2, source_codec=$3, source_fps=$4 WHERE id=$5`,
		width, height, codec, fps, id,
	)
	return err
}
