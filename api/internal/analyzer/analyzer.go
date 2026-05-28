// Package analyzer runs the analyze-video.js subprocess against a GCS URI and
// returns the structured JSON output.
package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type Runner struct {
	ScriptPath string
	NodeBin    string
	Timeout    time.Duration
}

func New(scriptPath string) *Runner {
	return &Runner{
		ScriptPath: scriptPath,
		NodeBin:    "node",
		Timeout:    6 * time.Minute,
	}
}

// Run executes `node analyze-video.js --uri <uri> --json-stdout` and returns
// the stdout as raw JSON. Captures stderr separately for error reporting.
func (r *Runner) Run(ctx context.Context, gcsURI string) (json.RawMessage, error) {
	cctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, r.NodeBin, r.ScriptPath, "--uri", gcsURI, "--json-stdout")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("analyze-video.js timeout after %s", r.Timeout)
		}
		return nil, fmt.Errorf("analyze-video.js failed: %w (stderr: %s)", err, truncate(stderr.String(), 500))
	}

	out := stdout.Bytes()
	if !json.Valid(out) {
		return nil, fmt.Errorf("analyze-video.js produced invalid JSON: %s", truncate(string(out), 200))
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
