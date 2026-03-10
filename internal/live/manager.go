package live

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"philos-video/internal/models"
	"philos-video/internal/repository"
)

// Manager coordinates live stream lifecycle: stream key validation, FFmpeg
// session management, viewer counting, and VOD conversion on stream end.
type Manager struct {
	streamKeyRepo  *repository.StreamKeyRepo
	liveStreamRepo *repository.LiveStreamRepo
	videoRepo      *repository.VideoRepo
	dataDir        string

	mu       sync.RWMutex
	sessions map[string]*transcodeSession // stream_id → session
}

func NewManager(
	streamKeyRepo *repository.StreamKeyRepo,
	liveStreamRepo *repository.LiveStreamRepo,
	videoRepo *repository.VideoRepo,
	dataDir string,
) *Manager {
	return &Manager{
		streamKeyRepo:  streamKeyRepo,
		liveStreamRepo: liveStreamRepo,
		videoRepo:      videoRepo,
		dataDir:        dataDir,
		sessions:       make(map[string]*transcodeSession),
	}
}

// StartStream validates the stream key, creates DB records, and starts FFmpeg.
func (m *Manager) StartStream(streamKey string) (*models.LiveStream, error) {
	sk, err := m.streamKeyRepo.GetByID(streamKey)
	if err != nil {
		return nil, fmt.Errorf("looking up stream key: %w", err)
	}
	if sk == nil || !sk.IsActive {
		return nil, fmt.Errorf("invalid or inactive stream key")
	}

	stream, err := m.liveStreamRepo.Create(sk.ID, sk.UserLabel)
	if err != nil {
		return nil, fmt.Errorf("creating live stream record: %w", err)
	}

	sess, err := newTranscodeSession(stream.ID, m.dataDir)
	if err != nil {
		_ = m.liveStreamRepo.UpdateStatus(stream.ID, models.StreamStatusEnded)
		return nil, fmt.Errorf("starting transcode session: %w", err)
	}

	m.mu.Lock()
	m.sessions[stream.ID] = sess
	m.mu.Unlock()

	if err := m.liveStreamRepo.UpdateStarted(stream.ID); err != nil {
		slog.Warn("updating stream started", "stream_id", stream.ID, "err", err)
	}

	// Re-fetch so caller gets updated status/timestamps.
	updated, _ := m.liveStreamRepo.GetByID(stream.ID)
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
func (m *Manager) EndStream(streamID string) {
	m.mu.Lock()
	sess := m.sessions[streamID]
	delete(m.sessions, streamID)
	m.mu.Unlock()

	if sess != nil {
		sess.stop()
	}

	if err := m.liveStreamRepo.UpdateEnded(streamID); err != nil {
		slog.Warn("marking stream ended", "stream_id", streamID, "err", err)
	}

	go m.convertToVOD(streamID)
}

func (m *Manager) convertToVOD(streamID string) {
	stream, err := m.liveStreamRepo.GetByID(streamID)
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
		ID:     videoID,
		Title:  stream.Title + " (Recording)",
		Status: models.VideoStatusReady,
	}
	if err := m.videoRepo.Create(v); err != nil {
		slog.Error("creating VOD video", "err", err)
		return
	}

	// HLS path is relative to data/hls — but live output is in data/live/{stream_id}.
	// We store the full relative path from dataDir.
	hlsPath := filepath.Join("live", streamID)
	if err := m.videoRepo.UpdateHLSPath(videoID, hlsPath); err != nil {
		slog.Error("setting VOD hls_path", "err", err)
		return
	}

	if err := m.liveStreamRepo.UpdateVideoID(streamID, videoID); err != nil {
		slog.Warn("linking VOD to stream", "err", err)
	}

	slog.Info("live stream converted to VOD", "stream_id", streamID, "video_id", videoID)
}

// GetStream returns a live stream by ID.
func (m *Manager) GetStream(id string) (*models.LiveStream, error) {
	return m.liveStreamRepo.GetByID(id)
}

// ListLive returns all currently live streams.
func (m *Manager) ListLive() ([]*models.LiveStream, error) {
	return m.liveStreamRepo.ListLive()
}

// ActiveCount returns the number of active transcode sessions.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
