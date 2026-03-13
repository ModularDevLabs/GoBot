package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handlePolicySimulate(w http.ResponseWriter, r *http.Request) {
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
		ActionType string   `json:"action_type"`
		UserIDs    []string `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	actionType := strings.TrimSpace(strings.ReplaceAll(payload.ActionType, "-", "_"))
	if actionType == "" || len(payload.UserIDs) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	settings, err := s.repos.Settings.Get(r.Context(), guildID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	incidentActive := settings.IncidentModeEnabled
	if incidentActive && settings.IncidentModeEndsAt != "" {
		if t, err := time.Parse(time.RFC3339, settings.IncidentModeEndsAt); err == nil && time.Now().UTC().After(t) {
			incidentActive = false
		}
	}

	results := make([]map[string]any, 0, len(payload.UserIDs))
	for _, rawUserID := range payload.UserIDs {
		userID := strings.TrimSpace(rawUserID)
		if userID == "" {
			continue
		}
		preflight, err := s.discord.PreflightAction(r.Context(), guildID, userID, actionType)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		notes := make([]string, 0, 4)
		if settings.ActionDryRun {
			notes = append(notes, "action_dry_run enabled")
		}
		if settings.ActionRequireConfirm || (incidentActive && (actionType == "kick" || actionType == "quarantine" || actionType == "remove_roles")) {
			notes = append(notes, "confirm token required")
		}
		if settings.ActionTwoPersonApproval || (incidentActive && (actionType == "kick" || actionType == "remove_roles")) {
			notes = append(notes, "distinct approver required")
		}
		results = append(results, map[string]any{
			"user_id":                  userID,
			"action_type":              actionType,
			"preflight":                preflight,
			"action_dry_run":           settings.ActionDryRun,
			"confirm_required":         settings.ActionRequireConfirm || (incidentActive && (actionType == "kick" || actionType == "quarantine" || actionType == "remove_roles")),
			"distinct_approver_needed": settings.ActionTwoPersonApproval || (incidentActive && (actionType == "kick" || actionType == "remove_roles")),
			"incident_mode_active":     incidentActive,
			"notes":                    notes,
		})
	}
	writeJSON(w, map[string]any{"results": results})
}
