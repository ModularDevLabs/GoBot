package web

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type caseTimelineItem struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Actor   string    `json:"actor"`
	Summary string    `json:"summary"`
}

func (s *Server) handleCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	guildID := r.URL.Query().Get("guild_id")
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if guildID == "" || userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), 200)
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	out := make([]caseTimelineItem, 0, limit)

	warnings, err := s.repos.Warnings.ListByGuild(r.Context(), guildID, limit)
	if err == nil {
		for _, row := range warnings {
			if row.UserID != userID {
				continue
			}
			out = append(out, caseTimelineItem{
				Time:    row.CreatedAt,
				Type:    "warning",
				Actor:   row.ActorUserID,
				Summary: row.Reason,
			})
		}
	}

	actions, err := s.repos.Actions.List(r.Context(), guildID, "", limit, 0)
	if err == nil {
		for _, row := range actions {
			if row.TargetUserID != userID {
				continue
			}
			out = append(out, caseTimelineItem{
				Time:    row.CreatedAt,
				Type:    "action_" + row.Type,
				Actor:   row.ActorUserID,
				Summary: fmt.Sprintf("status=%s %s", row.Status, strings.TrimSpace(row.Error)),
			})
		}
	}

	notes, err := s.repos.MemberNotes.List(r.Context(), guildID, userID, limit)
	if err == nil {
		for _, row := range notes {
			summary := row.Body
			if row.ResolvedAt != nil {
				summary = summary + " (resolved)"
			}
			out = append(out, caseTimelineItem{
				Time:    row.CreatedAt,
				Type:    "member_note",
				Actor:   row.AuthorID,
				Summary: summary,
			})
		}
	}

	appeals, err := s.repos.Appeals.ListByGuild(r.Context(), guildID, "", limit)
	if err == nil {
		for _, row := range appeals {
			if row.UserID != userID {
				continue
			}
			out = append(out, caseTimelineItem{
				Time:    row.CreatedAt,
				Type:    "appeal",
				Actor:   row.ReviewedBy,
				Summary: fmt.Sprintf("%s %s", row.Status, row.Reason),
			})
		}
	}

	tickets, err := s.repos.Tickets.ListByGuild(r.Context(), guildID, "", limit)
	if err == nil {
		for _, row := range tickets {
			if row.CreatorUserID != userID {
				continue
			}
			out = append(out, caseTimelineItem{
				Time:    row.CreatedAt,
				Type:    "ticket",
				Actor:   row.CreatorUserID,
				Summary: fmt.Sprintf("ticket #%d %s (%s)", row.ID, row.Status, row.Subject),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.After(out[j].Time)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	writeJSON(w, out)
}
