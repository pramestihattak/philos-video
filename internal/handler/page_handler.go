package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"philos-video/internal/live"
	"philos-video/internal/middleware"
	"philos-video/internal/models"
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
	return &PageHandler{
		videoSvc: videoSvc,
		liveMgr:  liveMgr,
		tmpl:     tmpl,
	}, nil
}

// GET /login
func (h *PageHandler) Login(w http.ResponseWriter, r *http.Request) {
	// If already signed in, redirect to home.
	if user := middleware.CurrentUser(r.Context()); user != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	returnURL := loginReturnURL(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "login.html", map[string]string{
		"ReturnURL": returnURL,
	})
}

// GET /
func (h *PageHandler) Library(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "library.html", map[string]any{
		"User": user,
	})
}

// GET /upload
func (h *PageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "upload.html", map[string]any{
		"User": user,
	})
}

// GET /dashboard
func (h *PageHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "dashboard.html", map[string]any{
		"User": user,
	})
}

// GET /watch/{video_id}
func (h *PageHandler) Watch(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("video_id")
	video, err := h.videoSvc.GetVideo(videoID)
	if err != nil || video == nil {
		http.NotFound(w, r)
		return
	}

	// Private videos require the signed-in owner.
	if video.Visibility == models.VisibilityPrivate {
		user := middleware.CurrentUser(r.Context())
		if user == nil || user.ID != video.UserID {
			http.Redirect(w, r, "/login?return=/watch/"+videoID, http.StatusFound)
			return
		}
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
	user := middleware.CurrentUser(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "go_live.html", map[string]any{
		"User": user,
	})
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
