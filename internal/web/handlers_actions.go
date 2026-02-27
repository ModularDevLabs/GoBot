package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ModularDevLabs/GoBot/internal/models"
)

func (s *Server) handleActions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		guildID := r.URL.Query().Get("guild_id")
		if guildID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		status := r.URL.Query().Get("status")
		limit := parseInt(r.URL.Query().Get("limit"), 50)
		offset := parseInt(r.URL.Query().Get("offset"), 0)

		rows, err := s.repos.Actions.List(r.Context(), guildID, status, limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.enrichActionTargets(r.Context(), guildID, rows)
		writeJSON(w, rows)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleActionDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/actions/")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(path, "quarantine") || strings.HasPrefix(path, "kick") || strings.HasPrefix(path, "remove-roles") {
		s.handleActionCreate(w, r)
		return
	}

	segments := strings.Split(path, "/")
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(segments) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		row, ok, err := s.repos.Actions.Get(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		rows := []models.ActionRow{row}
		s.enrichActionTargets(r.Context(), row.GuildID, rows)
		row = rows[0]
		writeJSON(w, row)
		return
	}

	if len(segments) == 2 && segments[1] == "retry" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := s.repos.Actions.UpdateStatus(r.Context(), id, "queued", ""); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.discord.NotifyActionQueued()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) enrichActionTargets(ctx context.Context, guildID string, rows []models.ActionRow) {
	for i := range rows {
		targetID := rows[i].TargetUserID
		if targetID == "" {
			continue
		}
		if member, ok, err := s.repos.Activity.GetMember(ctx, guildID, targetID); err == nil && ok {
			if member.DisplayName != "" {
				rows[i].TargetName = member.DisplayName
				continue
			}
			if member.Username != "" {
				rows[i].TargetName = member.Username
				continue
			}
		}
		rows[i].TargetName = s.discord.ResolveMemberDisplayName(guildID, targetID)
		if rows[i].TargetName == "" {
			var payload struct {
				TargetName string `json:"target_name"`
			}
			if err := json.Unmarshal([]byte(rows[i].PayloadJSON), &payload); err == nil && payload.TargetName != "" {
				rows[i].TargetName = payload.TargetName
			}
		}
	}
}

func (s *Server) handleActionCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/actions/")
	path = strings.TrimSuffix(path, "/")

	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		UserIDs              []string          `json:"user_ids"`
		Reason               string            `json:"reason"`
		RoleIDs              []string          `json:"role_ids"`
		RemoveAllExceptAllow bool              `json:"remove_all_except_allowlist"`
		TargetName           string            `json:"target_name"`
		TargetNames          map[string]string `json:"target_names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	actor := r.Header.Get("X-Actor-User")
	if actor == "" {
		actor = "dashboard"
	}

	actionType := strings.ReplaceAll(path, "-", "_")
	ids := make([]int64, 0, len(payload.UserIDs))
	for _, id := range payload.UserIDs {
		targetName := payload.TargetName
		if payload.TargetNames != nil {
			if name, ok := payload.TargetNames[id]; ok && name != "" {
				targetName = name
			}
		}
		row := models.ActionRow{
			GuildID:      guildID,
			ActorUserID:  actor,
			TargetUserID: id,
			Type:         actionType,
			PayloadJSON:  toJSON(map[string]any{"reason": payload.Reason, "role_ids": payload.RoleIDs, "remove_all_except_allowlist": payload.RemoveAllExceptAllow, "target_name": targetName}),
		}
		newID, err := s.repos.Actions.Enqueue(r.Context(), row)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		ids = append(ids, newID)
	}
	s.discord.NotifyActionQueued()
	writeJSON(w, map[string]any{"action_ids": ids})
}

func toJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
