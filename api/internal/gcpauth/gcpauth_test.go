package gcpauth

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestWriteCredsFile(t *testing.T) {
	want := []byte(`{"type":"service_account"}`)
	b64 := base64.StdEncoding.EncodeToString(want)

	tmp := t.TempDir() + "/sa.json"
	if err := WriteCredsFile(b64, tmp); err != nil {
		t.Fatalf("WriteCredsFile: %v", err)
	}
	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("contents mismatch: got %q want %q", got, want)
	}
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != tmp {
		t.Errorf("GOOGLE_APPLICATION_CREDENTIALS not set to %s", tmp)
	}
}

func TestWriteCredsFile_BadBase64(t *testing.T) {
	if err := WriteCredsFile("not-base64!!!", t.TempDir()+"/sa.json"); err == nil {
		t.Fatal("expected error on invalid base64")
	}
}
