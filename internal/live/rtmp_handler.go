package live

import (
	"io"
	"log/slog"

	rtmp "github.com/yutopp/go-rtmp"
	"github.com/yutopp/go-rtmp/message"
)

// rtmpHandler is an RTMP connection handler (one instance per OBS connection).
// It embeds DefaultHandler so only the methods we care about need implementing.
type rtmpHandler struct {
	rtmp.DefaultHandler
	manager  *Manager
	streamID string // set in OnPublish
}

func (h *rtmpHandler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *message.NetStreamPublish) error {
	streamKey := cmd.PublishingName
	masked := streamKey
	if len(masked) > 8 {
		masked = masked[:8] + "..."
	}
	slog.Info("RTMP publish", "stream_key", masked)

	stream, err := h.manager.StartStream(streamKey)
	if err != nil {
		slog.Error("starting stream", "stream_key", streamKey, "err", err)
		return err
	}
	h.streamID = stream.ID
	slog.Info("live stream started", "stream_id", stream.ID)
	return nil
}

func (h *rtmpHandler) OnVideo(timestamp uint32, payload io.Reader) error {
	if h.streamID == "" {
		return nil
	}
	return h.manager.WriteVideo(h.streamID, timestamp, payload)
}

func (h *rtmpHandler) OnAudio(timestamp uint32, payload io.Reader) error {
	if h.streamID == "" {
		return nil
	}
	return h.manager.WriteAudio(h.streamID, timestamp, payload)
}

func (h *rtmpHandler) OnClose() {
	if h.streamID != "" {
		slog.Info("RTMP connection closed, ending stream", "stream_id", h.streamID)
		h.manager.EndStream(h.streamID)
	}
}
