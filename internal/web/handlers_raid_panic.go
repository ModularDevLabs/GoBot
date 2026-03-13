package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleRaidPanicActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	guildID := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var payload struct {
		ActorUserID     string `json:"actor_user_id"`
		DurationMinutes int    `json:"duration_minutes"`
		SlowmodeSeconds int    `json:"slowmode_seconds"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	settings, _ := s.repos.Settings.Get(r.Context(), guildID)
	if payload.DurationMinutes <= 0 {
		payload.DurationMinutes = settings.RaidPanicDefaultMinutes
	}
	if payload.SlowmodeSeconds <= 0 {
		payload.SlowmodeSeconds = settings.RaidPanicSlowmodeSeconds
	}
	res, err := s.discord.ActivateRaidPanic(r.Context(), guildID, strings.TrimSpace(payload.ActorUserID), payload.DurationMinutes, payload.SlowmodeSeconds)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleRaidPanicDeactivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	guildID := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	res, err := s.discord.DeactivateRaidPanic(r.Context(), guildID, strings.TrimSpace(payload.Reason))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleRaidPanicStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	guildID := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	row, ok, err := s.discord.RaidPanicStatus(r.Context(), guildID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"active":   ok,
		"lockdown": row,
	})
}
