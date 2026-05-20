package db

import (
	"context"
	"os"
	"testing"
)

// testDB returns a DB connected to TEST_DATABASE_URL (or DATABASE_URL).
// Skips the test if neither is set so the suite is safe in offline envs.
func testDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("set TEST_DATABASE_URL or DATABASE_URL to run db tests")
	}
	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "TRUNCATE analyses")
		db.Close()
	})
	return db
}
