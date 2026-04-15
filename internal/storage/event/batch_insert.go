package event

import (
	"context"
	"fmt"
	"strings"

	"philos-video/internal/models"
	"philos-video/internal/storage"
)

func (r *Repo) BatchInsert(ctx context.Context, events []models.PlaybackEvent) error {
	if len(events) == 0 {
		return nil
	}

	const cols = `session_id, video_id, event_type, timestamp,
		segment_number, segment_quality, segment_bytes, download_time_ms, throughput_bps,
		current_quality, buffer_length, playback_position,
		rebuffer_duration_ms, quality_from, quality_to, error_code, error_message`
	const numCols = 17

	var placeholders []string
	var args []interface{}
	idx := 1

	for _, e := range events {
		row := make([]string, numCols)
		for i := range numCols {
			row[i] = fmt.Sprintf("$%d", idx+i)
		}
		placeholders = append(placeholders, "("+strings.Join(row, ",")+")")
		args = append(args,
			e.SessionID, e.VideoID, e.EventType, e.Timestamp,
			storage.Ni(e.SegmentNumber), storage.Ns(e.SegmentQuality), storage.Ni64(e.SegmentBytes),
			storage.Ni(e.DownloadTimeMs), storage.Ni64(e.ThroughputBps),
			storage.Ns(e.CurrentQuality), storage.Nf64(e.BufferLength), storage.Nf64(e.PlaybackPosition),
			storage.Ni(e.RebufferDurationMs), storage.Ns(e.QualityFrom), storage.Ns(e.QualityTo),
			storage.Ns(e.ErrorCode), storage.Ns(e.ErrorMessage),
		)
		idx += numCols
	}

	query := "INSERT INTO playback_events (" + cols + ") VALUES " + strings.Join(placeholders, ",")
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}
