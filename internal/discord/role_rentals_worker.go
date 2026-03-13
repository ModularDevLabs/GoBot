package discord

import (
	"context"
	"time"
)

func (s *Service) runRoleRentalsWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		rows, err := s.repos.RoleRentals.Due(ctx, time.Now().UTC(), 100)
		if err != nil {
			continue
		}
		for _, row := range rows {
			_ = s.session.GuildMemberRoleRemove(row.GuildID, row.UserID, row.RoleID)
			_ = s.repos.RoleRentals.MarkExpired(ctx, row.ID)
		}
	}
}
