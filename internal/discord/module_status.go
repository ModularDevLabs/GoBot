package discord

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"
)

type InviteTrackerStatus struct {
	HasManageGuild bool   `json:"has_manage_guild"`
	HasGuild       bool   `json:"has_guild"`
	BotUserID      string `json:"bot_user_id"`
}

func (s *Service) GetInviteTrackerStatus(_ context.Context, guildID string) (InviteTrackerStatus, error) {
	if s.session == nil || s.session.State == nil || s.session.State.User == nil {
		return InviteTrackerStatus{}, errors.New("discord session not ready")
	}
	botUserID := s.session.State.User.ID
	if botUserID == "" {
		return InviteTrackerStatus{}, errors.New("bot user id unavailable")
	}
	member, err := s.session.GuildMember(guildID, botUserID)
	if err != nil {
		return InviteTrackerStatus{}, err
	}
	roles, err := s.session.GuildRoles(guildID)
	if err != nil {
		return InviteTrackerStatus{}, err
	}
	perms := memberPermissions(guildID, member, roles)
	hasManageGuild := (perms&discordgo.PermissionManageServer) != 0 || (perms&discordgo.PermissionAdministrator) != 0
	return InviteTrackerStatus{
		HasManageGuild: hasManageGuild,
		HasGuild:       true,
		BotUserID:      botUserID,
	}, nil
}

func memberPermissions(guildID string, member *discordgo.Member, roles []*discordgo.Role) int64 {
	perms := int64(0)
	for _, role := range roles {
		if role.ID == guildID {
			perms |= role.Permissions
		}
	}
	memberRoles := map[string]struct{}{}
	for _, id := range member.Roles {
		memberRoles[id] = struct{}{}
	}
	for _, role := range roles {
		if _, ok := memberRoles[role.ID]; ok {
			perms |= role.Permissions
		}
	}
	return perms
}
