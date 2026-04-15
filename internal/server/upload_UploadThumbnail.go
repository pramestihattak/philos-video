package server

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxThumbnailSize = 10 << 20 // 10 MiB

// UploadThumbnail handles POST /api/v1/uploads/{upload_id}/thumbnail.
func (s *Server) UploadThumbnail(w http.ResponseWriter, r *http.Request, uploadId string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailSize)
	if err := r.ParseMultipartForm(maxThumbnailSize); err != nil {
		writeError(w, "invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		writeError(w, "thumbnail field missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		writeError(w, "thumbnail must be an image", http.StatusBadRequest)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	ext = strings.ToLower(ext)

	thumbsDir := filepath.Join(s.dataDir, "thumbnails")
	if err := os.MkdirAll(thumbsDir, 0o700); err != nil {
		slog.Error("create thumbnails dir", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	thumbPath := filepath.Join(thumbsDir, uploadId+ext)
	f, err := os.Create(thumbPath)
	if err != nil {
		slog.Error("create thumbnail file", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		slog.Error("write thumbnail", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	relPath := uploadId + ext
	if err := s.videoRepo.UpdateThumbnailPath(r.Context(), uploadId, relPath); err != nil {
		slog.Error("update thumbnail path", "upload_id", uploadId, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
