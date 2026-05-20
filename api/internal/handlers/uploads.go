package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/gcs"
)

type UploadsHandler struct {
	GCS *gcs.Client
}

type signedURLRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type signedURLResponse struct {
	PutURL    string `json:"put_url"`
	GCSURI    string `json:"gcs_uri"`
	ExpiresAt string `json:"expires_at"`
}

func (h *UploadsHandler) SignedURL(w http.ResponseWriter, r *http.Request) {
	var req signedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Filename == "" {
		writeError(w, http.StatusBadRequest, "filename required")
		return
	}
	if req.ContentType == "" {
		req.ContentType = "video/mp4"
	}
	if !strings.HasPrefix(req.ContentType, "video/") {
		writeError(w, http.StatusBadRequest, "content_type must be video/*")
		return
	}

	ext := strings.ToLower(path.Ext(req.Filename))
	if ext == "" {
		ext = ".mp4"
	}
	objectKey := time.Now().UTC().Format("2006-01-02") + "/" + randomHex(16) + ext

	url, expires, err := h.GCS.SignedPutURL(objectKey, req.ContentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign url: "+err.Error())
		return
	}
	gcsURI := "gs://" + h.GCS.Bucket() + "/" + objectKey

	writeJSON(w, http.StatusOK, signedURLResponse{
		PutURL:    url,
		GCSURI:    gcsURI,
		ExpiresAt: expires.UTC().Format(time.RFC3339),
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
