package web

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

var birthdayMMDD = regexp.MustCompile(`^\d{2}-\d{2}$`)

func (s *Server) handleBirthdays(w http.ResponseWriter, r *http.Request) {
	guildID := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.repos.Birthdays.ListByGuild(r.Context(), guildID, 500)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, rows)
	case http.MethodPost:
		var payload struct {
			UserID       string `json:"user_id"`
			BirthdayMMDD string `json:"birthday_mmdd"`
			Timezone     string `json:"timezone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		payload.UserID = strings.TrimSpace(payload.UserID)
		payload.BirthdayMMDD = strings.TrimSpace(payload.BirthdayMMDD)
		payload.Timezone = strings.TrimSpace(payload.Timezone)
		if payload.Timezone == "" {
			payload.Timezone = "UTC"
		}
		if payload.UserID == "" || !birthdayMMDD.MatchString(payload.BirthdayMMDD) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := s.repos.Birthdays.Upsert(r.Context(), guildID, payload.UserID, payload.BirthdayMMDD, payload.Timezone); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := s.repos.Birthdays.Delete(r.Context(), guildID, userID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
