// Package gcpauth decodes the base64-encoded service account JSON from the
// GOOGLE_APPLICATION_CREDENTIALS_JSON env var and writes it to disk so the
// GCP SDKs can pick it up via GOOGLE_APPLICATION_CREDENTIALS.
package gcpauth

import (
	"encoding/base64"
	"fmt"
	"os"
)

// WriteCredsFile decodes b64 into outPath and sets
// GOOGLE_APPLICATION_CREDENTIALS=outPath in the current process env.
func WriteCredsFile(b64, outPath string) error {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("decode credentials base64: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return fmt.Errorf("write credentials file: %w", err)
	}
	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", outPath); err != nil {
		return fmt.Errorf("setenv GOOGLE_APPLICATION_CREDENTIALS: %w", err)
	}
	return nil
}
