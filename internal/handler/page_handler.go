package handler

import (
	"html/template"
	"net/http"

	"philos-video/internal/live"
	"philos-video/internal/service"
	"philos-video/internal/web"
)

type PageHandler struct {
	videoSvc *service.VideoService
	liveMgr  *live.Manager
	tmpl     *template.Template
}

func NewPageHandler(videoSvc *service.VideoService, liveMgr *live.Manager) (*PageHandler, error) {
	tmpl, err := template.ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &PageHandler{videoSvc: videoSvc, liveMgr: liveMgr, tmpl: tmpl}, nil
}

// GET /
func (h *PageHandler) Library(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "library.html", nil)
}

// GET /upload
func (h *PageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "upload.html", nil)
}

// GET /dashboard
func (h *PageHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "dashboard.html", nil)
}

// GET /watch/{video_id}
func (h *PageHandler) Watch(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("video_id")
	video, err := h.videoSvc.GetVideo(videoID)
	if err != nil || video == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "player.html", map[string]string{
		"VideoID": videoID,
		"Title":   video.Title,
	})
}

// GET /go-live
func (h *PageHandler) GoLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "go_live.html", nil)
}

// GET /watch-live/{stream_id}
func (h *PageHandler) WatchLive(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("stream_id")
	stream, err := h.liveMgr.GetStream(streamID)
	if err != nil || stream == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "watch_live.html", map[string]string{
		"StreamID": streamID,
		"Title":    stream.Title,
	})
}
