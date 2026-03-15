package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ModularDevLabs/Fundamentum/internal/models"
)

type authContextKey string

const dashboardRoleContextKey authContextKey = "dashboard_role"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := s.authenticatedRole(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		ctx := context.WithValue(r.Context(), dashboardRoleContextKey, role)
		r = r.WithContext(ctx)
		if !s.authorizeRoleForRequest(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorizeRoleForRequest(w http.ResponseWriter, r *http.Request) bool {
	policyKey := rbacPolicyKeyForPath(r.URL.Path)
	if policyKey == "" {
		return true
	}
	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		return true
	}
	cfg, err := s.repos.Settings.Get(r.Context(), guildID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	role := strings.TrimSpace(strings.ToLower(dashboardRoleFromContext(r.Context())))
	if role == "" {
		role = "admin"
	}
	if role == "admin" {
		return true
	}
	allowed := cfg.DashboardRolePolicies[policyKey]
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if strings.TrimSpace(strings.ToLower(candidate)) == role {
			return true
		}
	}
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("forbidden by role policy"))
	return false
}

func dashboardRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(dashboardRoleContextKey).(string)
	return strings.TrimSpace(strings.ToLower(role))
}

func rbacPolicyKeyForPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/actions"):
		return "actions"
	case strings.HasPrefix(path, "/api/raid/panic"):
		return models.FeatureRaidPanic
	case strings.HasPrefix(path, "/api/settings"):
		return "settings"
	case strings.HasPrefix(path, "/api/modules/warnings"):
		return models.FeatureWarnings
	case strings.HasPrefix(path, "/api/modules/join-screening"):
		return models.FeatureJoinScreening
	case strings.HasPrefix(path, "/api/modules/reaction-roles"):
		return models.FeatureReactionRoles
	case strings.HasPrefix(path, "/api/modules/role-progression"):
		return models.FeatureRoleProgression
	case strings.HasPrefix(path, "/api/modules/scheduled"):
		return models.FeatureScheduled
	case strings.HasPrefix(path, "/api/modules/tickets"):
		return models.FeatureTickets
	case strings.HasPrefix(path, "/api/modules/appeals"):
		return models.FeatureAppeals
	case strings.HasPrefix(path, "/api/modules/custom-commands"):
		return models.FeatureCustomCommands
	case strings.HasPrefix(path, "/api/modules/birthdays"):
		return models.FeatureBirthdays
	case strings.HasPrefix(path, "/api/modules/streaks"):
		return models.FeatureStreaks
	case strings.HasPrefix(path, "/api/modules/season-resets"):
		return models.FeatureSeasonResets
	case strings.HasPrefix(path, "/api/modules/reputation"):
		return models.FeatureReputation
	case strings.HasPrefix(path, "/api/modules/economy"):
		return models.FeatureEconomy
	case strings.HasPrefix(path, "/api/modules/achievements"):
		return models.FeatureAchievements
	case strings.HasPrefix(path, "/api/modules/trivia"):
		return models.FeatureTrivia
	case strings.HasPrefix(path, "/api/modules/calendar"):
		return models.FeatureCalendar
	case strings.HasPrefix(path, "/api/modules/confessions"):
		return models.FeatureConfessions
	case strings.HasPrefix(path, "/api/modules/giveaways"):
		return models.FeatureGiveaways
	case strings.HasPrefix(path, "/api/modules/polls"):
		return models.FeaturePolls
	case strings.HasPrefix(path, "/api/modules/suggestions"):
		return models.FeatureSuggestions
	case strings.HasPrefix(path, "/api/modules/reminders"):
		return models.FeatureReminders
	case strings.HasPrefix(path, "/api/modules/member-notes"):
		return models.FeatureMemberNotes
	case strings.HasPrefix(path, "/api/modules/invite"):
		return models.FeatureInviteTracker
	default:
		return ""
	}
}

func (s *Server) authenticatedRole(r *http.Request) (string, bool) {
	secret := ""
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		secret = strings.TrimPrefix(auth, "Bearer ")
	}
	if secret == "" {
		cookie, err := r.Cookie("modbot_auth")
		if err == nil {
			secret = cookie.Value
		}
	}
	if secret == "" {
		return "", false
	}
	role, ok := s.roleFromSecret(secret)
	if !ok {
		return "", false
	}
	return role, true
}

func (s *Server) roleFromSecret(secret string) (string, bool) {
	if secret == s.adminPass {
		return "admin", true
	}
	for role, candidate := range s.dashboardRoleSecrets {
		if strings.TrimSpace(secret) == strings.TrimSpace(candidate) {
			normalized := strings.TrimSpace(strings.ToLower(role))
			if normalized == "" || normalized == "admin" {
				continue
			}
			return normalized, true
		}
	}
	return "", false
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}
