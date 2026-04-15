package server

import (
	"net/http"

	"philos-video/gen/api"
)

// ListLiveStreams handles GET /api/v1/live.
func (s *Server) ListLiveStreams(w http.ResponseWriter, r *http.Request) {
	streams, err := s.liveMgr.ListLive()
	if err != nil {
		writeError(w, "failed to list streams", http.StatusInternalServerError)
		return
	}
	if streams == nil {
		writeJSON(w, []api.ResponseLiveStream{}, http.StatusOK)
		return
	}
	writeJSON(w, toResponseLiveStreams(streams), http.StatusOK)
}
