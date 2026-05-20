package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	Uploads        *UploadsHandler
	Analyze        *AnalyzeHandler
	Analyses       *AnalysesHandler
	Auth           *AuthHandler
	AllowedOrigins []string
	JWTSecret      string
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(corsMiddleware(d.AllowedOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		// public auth routes
		r.Post("/auth/register", d.Auth.Register)
		r.Post("/auth/login", d.Auth.Login)

		// protected routes
		r.Group(func(r chi.Router) {
			r.Use(jwtMiddleware(d.JWTSecret))
			r.Get("/auth/me", d.Auth.Me)
			r.Put("/auth/me", d.Auth.UpdateMe)
			r.Post("/uploads/signed-url", d.Uploads.SignedURL)
			r.Post("/analyze", d.Analyze.Start)
			r.Get("/analyze/{id}", d.Analyze.Get)
			r.Get("/analyses", d.Analyses.List)
		})
	})

	return r
}

func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	allowedSet := map[string]bool{}
	for _, o := range allowed {
		allowedSet[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowedSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
