package handler

import (
	"html/template"
	"net/http"

	"philos-video/internal/service"
	"philos-video/internal/web"
)

type PageHandler struct {
	videoSvc *service.VideoService
	tmpl     *template.Template
}

func NewPageHandler(videoSvc *service.VideoService) (*PageHandler, error) {
	tmpl, err := template.ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &PageHandler{videoSvc: videoSvc, tmpl: tmpl}, nil
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
