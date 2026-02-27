package web

import "net/http"

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), 200)
	if limit > 1000 {
		limit = 1000
	}

	provider, ok := s.logger.(EventLogger)
	if !ok {
		writeJSON(w, []string{})
		return
	}
	writeJSON(w, provider.RecentEvents(limit))
}
