package discord

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) runModSummaryWorker(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	lastSent := map[string]time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		guilds, err := s.ListGuilds(ctx)
		if err != nil {
			continue
		}
		now := time.Now().UTC()
		for _, g := range guilds {
			settings, err := s.repos.Settings.Get(ctx, g.ID)
			if err != nil || settings.ModSummaryChannelID == "" {
				continue
			}
			if settings.InMaintenanceWindow(now) {
				continue
			}
			interval := time.Duration(settings.ModSummaryIntervalHours) * time.Hour
			if interval <= 0 {
				interval = 24 * time.Hour
			}
			if last, ok := lastSent[g.ID]; ok && now.Sub(last) < interval {
				continue
			}
			if err := s.sendModSummary(ctx, g.ID, settings.ModSummaryChannelID, now.Add(-interval), now); err != nil {
				s.logger.Error("mod summary send failed guild=%s err=%v", g.ID, err)
				continue
			}
			lastSent[g.ID] = now
		}
	}
}

func (s *Service) sendModSummary(ctx context.Context, guildID, channelID string, since, until time.Time) error {
	msg, err := s.modSummaryText(ctx, guildID, since, until)
	if err != nil {
		return err
	}
	_, err = s.session.ChannelMessageSend(channelID, msg)
	return err
}

func (s *Service) modSummaryText(ctx context.Context, guildID string, since, until time.Time) (string, error) {
	warnings, err := s.repos.Warnings.CountSince(ctx, guildID, since)
	if err != nil {
		return "", err
	}
	actions, err := s.repos.Actions.CountSince(ctx, guildID, since, "")
	if err != nil {
		return "", err
	}
	failed, err := s.repos.Actions.CountSince(ctx, guildID, since, "failed")
	if err != nil {
		return "", err
	}
	tickets, err := s.repos.Tickets.CountCreatedSince(ctx, guildID, since)
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("Mod summary (%s to %s)\nWarnings: %d\nActions: %d (failed=%d)\nTickets opened: %d",
		since.Format("2006-01-02 15:04"),
		until.Format("2006-01-02 15:04"),
		warnings, actions, failed, tickets,
	)
	return msg, nil
}

func (s *Service) GenerateModSummary(ctx context.Context, guildID string, since, until time.Time) (string, error) {
	return s.modSummaryText(ctx, guildID, since, until)
}
