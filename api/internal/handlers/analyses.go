package handlers

import (
	"net/http"

	"github.com/leoklaser/video-analyzer/api/internal/db"
)

type AnalysesHandler struct {
	DB *db.DB
}

func (h *AnalysesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r)
	items, err := db.List(r.Context(), h.DB, userID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}
