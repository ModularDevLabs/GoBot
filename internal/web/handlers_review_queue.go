package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleReviewQueue(w http.ResponseWriter, r *http.Request) {
	guildID := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.repos.Actions.List(r.Context(), guildID, "review_pending", 200, 0)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.enrichActionTargets(r.Context(), guildID, rows)
		writeJSON(w, rows)
	case http.MethodPost:
		var payload struct {
			ActionID int64  `json:"action_id"`
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decision := strings.ToLower(strings.TrimSpace(payload.Decision))
		if payload.ActionID <= 0 {
			if raw := strings.TrimSpace(r.URL.Query().Get("action_id")); raw != "" {
				if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
					payload.ActionID = id
				}
			}
		}
		if payload.ActionID <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch decision {
		case "approve":
			if err := s.repos.Actions.UpdateStatus(r.Context(), payload.ActionID, "queued", ""); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			s.discord.NotifyActionQueued()
		case "reject":
			reason := strings.TrimSpace(payload.Reason)
			if reason == "" {
				reason = "rejected by reviewer"
			}
			if err := s.repos.Actions.UpdateStatus(r.Context(), payload.ActionID, "failed", reason); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
