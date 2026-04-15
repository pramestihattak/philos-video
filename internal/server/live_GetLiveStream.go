package server

import "net/http"

// GetLiveStream handles GET /api/v1/live/{stream_id}.
func (s *Server) GetLiveStream(w http.ResponseWriter, r *http.Request, streamId string) {
	stream, err := s.liveMgr.GetStream(streamId)
	if err != nil || stream == nil {
		writeError(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, toResponseLiveStream(stream), http.StatusOK)
}
