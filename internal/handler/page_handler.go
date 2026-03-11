package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"philos-video/internal/live"
	"philos-video/internal/middleware"
	"philos-video/internal/service"
	"philos-video/internal/web"
)

type PageHandler struct {
	videoSvc  *service.VideoService
	liveMgr   *live.Manager
	tmpl      *template.Template
	goLivePin string
	jwtSecret string
}

func NewPageHandler(videoSvc *service.VideoService, liveMgr *live.Manager, goLivePin, jwtSecret string) (*PageHandler, error) {
	tmpl, err := template.ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &PageHandler{
		videoSvc:  videoSvc,
		liveMgr:   liveMgr,
		tmpl:      tmpl,
		goLivePin: goLivePin,
		jwtSecret: jwtSecret,
	}, nil
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
		"VideoID":   videoID,
		"Title":     video.Title,
		"PlayCount": fmt.Sprintf("%d", video.PlayCount),
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

// GET /go-live/login
func (h *PageHandler) GoLiveLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "go_live_login.html", map[string]string{"Error": ""})
}

// POST /go-live/login
func (h *PageHandler) GoLiveLoginPost(w http.ResponseWriter, r *http.Request) {
	if h.goLivePin == "" {
		http.Redirect(w, r, "/go-live", http.StatusFound)
		return
	}
	pin := r.FormValue("pin")
	if pin != h.goLivePin {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		h.tmpl.ExecuteTemplate(w, "go_live_login.html", map[string]string{"Error": "Incorrect PIN."})
		return
	}
	middleware.SetGoLiveCookie(w, h.goLivePin, h.jwtSecret)
	http.Redirect(w, r, "/go-live", http.StatusFound)
}
