package web

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleModSummaryGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	guildID := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	hours := parseInt(r.URL.Query().Get("hours"), 24)
	if hours <= 0 {
		hours = 24
	}
	now := time.Now().UTC()
	text, err := s.discord.GenerateModSummary(r.Context(), guildID, now.Add(-time.Duration(hours)*time.Hour), now)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"summary": text})
}
