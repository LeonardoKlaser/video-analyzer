package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/leoklaser/video-analyzer/api/internal/auth"
)

type ctxKey string

const ctxUserID ctxKey = "user_id"

func jwtMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing token")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			userID, err := auth.ParseToken(secret, tokenStr)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func userIDFromCtx(r *http.Request) uuid.UUID {
	if id, ok := r.Context().Value(ctxUserID).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
