package live

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"philos-video/internal/metrics"
	"philos-video/internal/models"
	"philos-video/internal/storage"
)

// Manager coordinates live stream lifecycle: stream key validation, FFmpeg
// session management, viewer counting, and VOD conversion on stream end.
type Manager struct {
	streamKeyRepo  storage.StreamKeyStorer
	liveStreamRepo storage.LiveStreamStorer
	videoRepo      storage.VideoStorer
	dataDir        string

	mu         sync.RWMutex
	sessions   map[string]*transcodeSession // stream_id → session
	startTimes map[string]time.Time         // stream_id → start time
}

func NewManager(
	streamKeyRepo storage.StreamKeyStorer,
	liveStreamRepo storage.LiveStreamStorer,
	videoRepo storage.VideoStorer,
	dataDir string,
) *Manager {
	return &Manager{
		streamKeyRepo:  streamKeyRepo,
		liveStreamRepo: liveStreamRepo,
		videoRepo:      videoRepo,
		dataDir:        dataDir,
		sessions:       make(map[string]*transcodeSession),
		startTimes:     make(map[string]time.Time),
	}
}

const maxLiveStreams = 10

// StartStream validates the stream key, creates DB records, and starts FFmpeg.
// Called from the RTMP handler — no request context available, so we use Background.
func (m *Manager) StartStream(streamKey string) (*models.LiveStream, error) {
	ctx := context.Background()

	m.mu.RLock()
	activeCount := len(m.sessions)
	m.mu.RUnlock()
	if activeCount >= maxLiveStreams {
		metrics.RTMPConnectionsTotal.WithLabelValues("rejected").Inc()
		return nil, fmt.Errorf("max concurrent streams (%d) reached", maxLiveStreams)
	}

	sk, err := m.streamKeyRepo.GetByID(ctx, streamKey)
	if err != nil {
		metrics.RTMPConnectionsTotal.WithLabelValues("rejected").Inc()
		return nil, fmt.Errorf("looking up stream key: %w", err)
	}
	if sk == nil || !sk.IsActive {
		metrics.RTMPConnectionsTotal.WithLabelValues("rejected").Inc()
		return nil, fmt.Errorf("invalid or inactive stream key")
	}

	stream, err := m.liveStreamRepo.Create(ctx, sk.ID, sk.UserLabel, sk.RecordVOD, sk.UserID)
	if err != nil {
		return nil, fmt.Errorf("creating live stream record: %w", err)
	}

	sess, err := newTranscodeSession(stream.ID, m.dataDir)
	if err != nil {
		_ = m.liveStreamRepo.UpdateStatus(ctx, stream.ID, models.StreamStatusEnded)
		return nil, fmt.Errorf("starting transcode session: %w", err)
	}

	m.mu.Lock()
	m.sessions[stream.ID] = sess
	m.startTimes[stream.ID] = time.Now()
	m.mu.Unlock()

	metrics.LiveStreamsActive.Inc()
	metrics.RTMPConnectionsTotal.WithLabelValues("accepted").Inc()

	if err := m.liveStreamRepo.UpdateStarted(ctx, stream.ID); err != nil {
		slog.Warn("updating stream started", "stream_id", stream.ID, "err", err)
	}

	// Re-fetch so caller gets updated status/timestamps.
	updated, _ := m.liveStreamRepo.GetByID(ctx, stream.ID)
	if updated != nil {
		return updated, nil
	}
	return stream, nil
}

// WriteVideo forwards an RTMP video packet to the FFmpeg transcode session.
func (m *Manager) WriteVideo(streamID string, timestamp uint32, payload interface{ Read([]byte) (int, error) }) error {
	m.mu.RLock()
	sess := m.sessions[streamID]
	m.mu.RUnlock()
	if sess == nil {
		return nil
	}
	return sess.writeVideo(timestamp, payload)
}

// WriteAudio forwards an RTMP audio packet to the FFmpeg transcode session.
func (m *Manager) WriteAudio(streamID string, timestamp uint32, payload interface{ Read([]byte) (int, error) }) error {
	m.mu.RLock()
	sess := m.sessions[streamID]
	m.mu.RUnlock()
	if sess == nil {
		return nil
	}
	return sess.writeAudio(timestamp, payload)
}

// EndStream stops transcoding, marks stream ended, and schedules VOD conversion.
// Called from the RTMP handler — no request context available, so we use Background.
func (m *Manager) EndStream(streamID string) {
	ctx := context.Background()

	m.mu.Lock()
	sess := m.sessions[streamID]
	startTime := m.startTimes[streamID]
	delete(m.sessions, streamID)
	delete(m.startTimes, streamID)
	m.mu.Unlock()

	if sess != nil {
		sess.stop()
	}

	metrics.LiveStreamsActive.Dec()
	metrics.LiveStreamsTotal.WithLabelValues("normal").Inc()
	if !startTime.IsZero() {
		metrics.LiveStreamDuration.Observe(time.Since(startTime).Seconds())
	}

	if err := m.liveStreamRepo.UpdateEnded(ctx, streamID); err != nil {
		slog.Warn("marking stream ended", "stream_id", streamID, "err", err)
	}

	stream, err := m.liveStreamRepo.GetByID(ctx, streamID)
	if err != nil {
		slog.Warn("fetching stream for VOD decision", "stream_id", streamID, "err", err)
		return
	}
	if stream != nil && stream.RecordVOD {
		go m.convertToVOD(streamID)
	} else {
		slog.Info("stream ended without recording (record_vod=false)", "stream_id", streamID)
	}
}

func (m *Manager) convertToVOD(streamID string) {
	ctx := context.Background()

	stream, err := m.liveStreamRepo.GetByID(ctx, streamID)
	if err != nil || stream == nil {
		slog.Warn("converting to VOD: stream not found", "stream_id", streamID)
		return
	}

	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		slog.Error("generating VOD video ID", "err", err)
		return
	}
	videoID := hex.EncodeToString(b)

	v := &models.Video{
		ID:         videoID,
		UserID:     stream.UserID,
		Title:      stream.Title + " (Recording)",
		Visibility: models.VisibilityPrivate,
		Status:     models.VideoStatusReady,
	}
	if err := m.videoRepo.Create(ctx, v); err != nil {
		slog.Error("creating VOD video", "err", err)
		return
	}

	// HLS path is relative to data/hls — but live output is in data/live/{stream_id}.
	// We store the full relative path from dataDir.
	hlsPath := filepath.Join("live", streamID)
	if err := m.videoRepo.UpdateHLSPath(ctx, videoID, hlsPath); err != nil {
		slog.Error("setting VOD hls_path", "err", err)
		return
	}

	if err := m.liveStreamRepo.UpdateVideoID(ctx, streamID, videoID); err != nil {
		slog.Warn("linking VOD to stream", "err", err)
	}

	slog.Info("live stream converted to VOD", "stream_id", streamID, "video_id", videoID)
}

// GetStream returns a live stream by ID.
func (m *Manager) GetStream(id string) (*models.LiveStream, error) {
	return m.liveStreamRepo.GetByID(context.Background(), id)
}

// ListLive returns all currently live streams.
func (m *Manager) ListLive() ([]*models.LiveStream, error) {
	return m.liveStreamRepo.ListLive(context.Background())
}

// ActiveCount returns the number of active transcode sessions.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// GetPIDs returns a map of stream_id → FFmpeg process PID for all active sessions.
func (m *Manager) GetPIDs() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]int, len(m.sessions))
	for id, sess := range m.sessions {
		if sess.cmd != nil && sess.cmd.Process != nil {
			out[id] = sess.cmd.Process.Pid
		}
	}
	return out
}

// EndAllStreams gracefully terminates every active live stream.
// Call this during server shutdown to ensure #EXT-X-ENDLIST is written.
func (m *Manager) EndAllStreams() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		slog.Info("ending stream on shutdown", "stream_id", id)
		m.EndStream(id)
	}
}
