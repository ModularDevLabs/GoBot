package discord

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type ActionPreflightIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type ActionPreflightResult struct {
	GuildID      string                 `json:"guild_id"`
	TargetUserID string                 `json:"target_user_id"`
	ActionType   string                 `json:"action_type"`
	Allowed      bool                   `json:"allowed"`
	Issues       []ActionPreflightIssue `json:"issues"`
}

func (s *Service) PreflightAction(ctx context.Context, guildID, targetUserID, actionType string) (ActionPreflightResult, error) {
	result := ActionPreflightResult{
		GuildID:      guildID,
		TargetUserID: targetUserID,
		ActionType:   actionType,
		Allowed:      true,
		Issues:       []ActionPreflightIssue{},
	}
	addIssue := func(severity, code, msg string) {
		result.Issues = append(result.Issues, ActionPreflightIssue{
			Severity: severity,
			Code:     code,
			Message:  msg,
		})
		if severity == "error" {
			result.Allowed = false
		}
	}

	settings, err := s.repos.Settings.Get(ctx, guildID)
	if err != nil {
		return result, err
	}
	incidentActive := settings.IncidentModeEnabled
	if incidentActive && settings.IncidentModeEndsAt != "" {
		if t, err := time.Parse(time.RFC3339, settings.IncidentModeEndsAt); err == nil && time.Now().UTC().After(t) {
			incidentActive = false
		}
	}

	guild, err := s.session.Guild(guildID)
	if err != nil || guild == nil {
		return result, fmt.Errorf("load guild: %w", err)
	}
	if guild.OwnerID == targetUserID {
		addIssue("error", "target_is_owner", "Cannot moderate the server owner.")
		return result, nil
	}

	if s.session.State == nil || s.session.State.User == nil || s.session.State.User.ID == "" {
		addIssue("error", "bot_unavailable", "Bot session not ready.")
		return result, nil
	}
	botUserID := s.session.State.User.ID
	botMember, err := s.session.GuildMember(guildID, botUserID)
	if err != nil {
		addIssue("error", "bot_member_unavailable", "Unable to inspect bot role hierarchy in this guild.")
		return result, nil
	}
	targetMember, err := s.session.GuildMember(guildID, targetUserID)
	if err != nil {
		addIssue("error", "target_member_unavailable", "Unable to inspect target member role hierarchy.")
		return result, nil
	}
	roles, err := s.session.GuildRoles(guildID)
	if err != nil {
		addIssue("error", "roles_unavailable", "Unable to load guild role list.")
		return result, nil
	}

	rolePos := map[string]int{}
	for _, role := range roles {
		rolePos[role.ID] = role.Position
	}
	maxRolePosition := func(roleIDs []string) int {
		max := -1
		for _, roleID := range roleIDs {
			if p, ok := rolePos[roleID]; ok && p > max {
				max = p
			}
		}
		return max
	}
	botTop := maxRolePosition(botMember.Roles)
	targetTop := maxRolePosition(targetMember.Roles)
	if targetTop >= botTop {
		addIssue("error", "role_hierarchy_block", "Target member top role is higher or equal to bot top role.")
	}

	isAdmin, err := s.memberIsAdmin(guildID, targetUserID)
	if err == nil && isAdmin {
		switch settings.AdminUserPolicy {
		case "refuse":
			addIssue("error", "admin_policy_refuse", "Target has Administrator permission and policy is set to refuse.")
		case "remove_admin_roles":
			addIssue("warning", "admin_role_strip", "Target has Administrator permission; admin roles will be removed before action.")
		default:
			addIssue("warning", "admin_target", "Target has Administrator permission; action may still fail depending on server protections.")
		}
	}

	switch actionType {
	case "kick":
		addIssue("warning", "kick_irreversible", "Kick is destructive. Confirm reason and target before queueing.")
		if incidentActive {
			addIssue("warning", "incident_mode_extra_review", "Incident mode is active: use two-person approval for destructive actions.")
		}
	case "quarantine":
		if settings.QuarantineRoleID == "" {
			addIssue("warning", "quarantine_role_auto", "Quarantine role not set explicitly; bot will attempt auto-provisioning.")
		}
		if incidentActive {
			addIssue("warning", "incident_mode_extra_review", "Incident mode is active: quarantine actions should include clear reasoning.")
		}
	case "remove_roles":
		if incidentActive {
			addIssue("warning", "incident_mode_extra_review", "Incident mode is active: use two-person approval for destructive actions.")
		}
	default:
		addIssue("warning", "unknown_action", "Unknown action type provided to preflight.")
	}

	sort.SliceStable(result.Issues, func(i, j int) bool {
		return result.Issues[i].Severity == "error" && result.Issues[j].Severity != "error"
	})
	return result, nil
}
