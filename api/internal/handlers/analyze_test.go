package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStart_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", ``, "invalid JSON"},
		{"missing gcs_uri", `{"mode":"pre_post","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "gcs_uri"},
		{"wrong bucket", `{"gcs_uri":"gs://other-bucket/x.mp4","mode":"pre_post","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "gcs_uri"},
		{"bad mode", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"weird","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "mode"},
		{"empty business_context.brand_name", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"pre_post","business_context":{"description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "brand_name"},
		{"metrics without post_mortem", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"pre_post","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"},"metrics":{"views":1}}`, "metrics"},
	}
	h := &AnalyzeHandler{Bucket: "video-analyzer-tmp"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/analyze", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Start(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body missing %q: %s", tc.want, rec.Body.String())
			}
		})
	}
}
