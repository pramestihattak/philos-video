package logging

import (
	"context"
	"log/slog"
)

type logKey string

const (
	videoIDKey   logKey = "video_id"
	sessionIDKey logKey = "session_id"
	streamIDKey  logKey = "stream_id"
	jobIDKey     logKey = "job_id"
	requestIDKey logKey = "request_id"
)

// FromContext returns slog.Default() enriched with any context fields that are set.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if v, _ := ctx.Value(requestIDKey).(string); v != "" {
		logger = logger.With("request_id", v)
	}
	if v, _ := ctx.Value(videoIDKey).(string); v != "" {
		logger = logger.With("video_id", v)
	}
	if v, _ := ctx.Value(sessionIDKey).(string); v != "" {
		logger = logger.With("session_id", v)
	}
	if v, _ := ctx.Value(streamIDKey).(string); v != "" {
		logger = logger.With("stream_id", v)
	}
	if v, _ := ctx.Value(jobIDKey).(string); v != "" {
		logger = logger.With("job_id", v)
	}
	return logger
}

// WithVideoID returns a new context with the video_id set.
func WithVideoID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, videoIDKey, id)
}

// WithSessionID returns a new context with the session_id set.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// WithStreamID returns a new context with the stream_id set.
func WithStreamID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, streamIDKey, id)
}

// WithJobID returns a new context with the job_id set.
func WithJobID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, jobIDKey, id)
}

// WithRequestID returns a new context with the request_id set.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
