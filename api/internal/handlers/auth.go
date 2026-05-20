package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/leoklaser/video-analyzer/api/internal/auth"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

type AuthHandler struct {
	DB        *db.DB
	JWTSecret string
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "email and password (min 6 chars) required")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	u, err := db.CreateUser(r.Context(), h.DB, req.Email, hash)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	token, err := auth.SignToken(h.JWTSecret, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not sign token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": u})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	u, hash, err := db.GetUserByEmail(r.Context(), h.DB, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !auth.CheckPassword(hash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.SignToken(h.JWTSecret, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not sign token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r)
	u, err := db.GetUserByID(r.Context(), h.DB, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var bc models.BusinessContext
	if err := json.NewDecoder(r.Body).Decode(&bc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	userID := userIDFromCtx(r)
	if err := db.UpdateUserBusinessContext(r.Context(), h.DB, userID, &bc); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
