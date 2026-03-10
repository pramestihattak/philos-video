package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"philos-video/internal/models"
)

type EventRepo struct {
	db *sql.DB
}

func NewEventRepo(db *sql.DB) *EventRepo {
	return &EventRepo{db: db}
}

func (r *EventRepo) BatchInsert(ctx context.Context, events []models.PlaybackEvent) error {
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
			ni(e.SegmentNumber), ns(e.SegmentQuality), ni64(e.SegmentBytes),
			ni(e.DownloadTimeMs), ni64(e.ThroughputBps),
			ns(e.CurrentQuality), nf64(e.BufferLength), nf64(e.PlaybackPosition),
			ni(e.RebufferDurationMs), ns(e.QualityFrom), ns(e.QualityTo),
			ns(e.ErrorCode), ns(e.ErrorMessage),
		)
		idx += numCols
	}

	query := "INSERT INTO playback_events (" + cols + ") VALUES " + strings.Join(placeholders, ",")
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// nullable helpers shared across repo package
func ns(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func ni(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func ni64(p *int64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func nf64(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

