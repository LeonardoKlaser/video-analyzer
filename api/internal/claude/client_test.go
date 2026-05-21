package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Analyze_Happy(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "sk-test" {
			t.Errorf("x-api-key: got %q want sk-test", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version: got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"{\"hook_analysis\":{\"score\":7,\"why\":\"w\",\"improvement\":\"i\"},\"structure_analysis\":{\"framework_match\":\"x\",\"retention_issues\":[]},\"visual_analysis\":{\"rhythm\":\"fast\",\"first_frame\":\"y\",\"dominant_visual\":\"\"},\"key_insights\":[\"a\"],\"action_items\":[\"a\"],\"verdict\":\"ok\",\"verdict_reason\":\"r\"}"}]
		}`))
	}))
	defer server.Close()

	c := NewClient("sk-test", "claude-sonnet-4-6", server.URL)
	raw, err := c.Analyze(context.Background(), "system text", "user text")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !strings.Contains(capturedBody, `"system":"system text"`) {
		t.Errorf("request body missing system: %s", capturedBody)
	}

	res, err := ParseResult(raw)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if res.Verdict != "ok" {
		t.Errorf("Verdict: %q", res.Verdict)
	}
}

func TestClient_Analyze_RetriesOnceOn5xx(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"verdict\":\"ok\",\"verdict_reason\":\"\",\"hook_analysis\":{\"score\":1,\"why\":\"w\",\"improvement\":\"\"},\"structure_analysis\":{\"framework_match\":\"\",\"retention_issues\":[]},\"visual_analysis\":{\"rhythm\":\"\",\"first_frame\":\"\",\"dominant_visual\":\"\"},\"key_insights\":[\"a\"],\"action_items\":[\"a\"]}"}]}`))
	}))
	defer server.Close()

	c := NewClient("sk", "m", server.URL)
	c.retryDelay = 0
	if _, err := c.Analyze(context.Background(), "s", "u"); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls: %d want 2", calls)
	}
}

func TestClient_Analyze_GivesUpAfterRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient("sk", "m", server.URL)
	c.retryDelay = 0
	if _, err := c.Analyze(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected error after retry exhausted")
	}
}

func TestExtractText(t *testing.T) {
	var resp anthropicResponse
	_ = json.Unmarshal([]byte(`{"content":[{"type":"text","text":"hello"}]}`), &resp)
	got := extractText(resp)
	if got != "hello" {
		t.Errorf("extractText: %q", got)
	}
}
