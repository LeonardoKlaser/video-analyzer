package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultEndpoint = "https://api.anthropic.com/v1/messages"

type Client struct {
	apiKey     string
	model      string
	endpoint   string
	httpClient *http.Client
	retryDelay time.Duration
}

func NewClient(apiKey, model, endpoint string) *Client {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Client{
		apiKey:     apiKey,
		model:      model,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 4 * time.Minute},
		retryDelay: 5 * time.Second,
	}
}

type anthropicRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []reqMessage `json:"messages"`
}

type reqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Analyze sends system + user prompt, returns raw JSON text from Claude.
// Retries once on 5xx after retryDelay.
func (c *Client) Analyze(ctx context.Context, system, user string) ([]byte, error) {
	body, _ := json.Marshal(anthropicRequest{
		Model:     c.model,
		MaxTokens: 8192,
		System:    system,
		Messages:  []reqMessage{{Role: "user", Content: user}},
	})

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}
		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http: %w", err)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("claude %d: %s", resp.StatusCode, truncate(string(raw), 200))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("claude %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}

		var aresp anthropicResponse
		if err := json.Unmarshal(raw, &aresp); err != nil {
			return nil, fmt.Errorf("unmarshal claude resp: %w (body: %s)", err, truncate(string(raw), 200))
		}
		if aresp.Error != nil {
			return nil, fmt.Errorf("claude error: %s — %s", aresp.Error.Type, aresp.Error.Message)
		}
		text := extractText(aresp)
		text = stripJSONFence(text)
		return []byte(text), nil
	}
	return nil, lastErr
}

func extractText(r anthropicResponse) string {
	var b strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
