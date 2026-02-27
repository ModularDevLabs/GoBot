package web

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isAuthorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	})
}

func (s *Server) isAuthorized(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		if strings.TrimPrefix(auth, "Bearer ") == s.adminPass {
			return true
		}
	}
	cookie, err := r.Cookie("modbot_auth")
	if err == nil && cookie.Value == s.adminPass {
		return true
	}
	return false
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}
