package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/analyzer"
	"github.com/leoklaser/video-analyzer/api/internal/claude"
	"github.com/leoklaser/video-analyzer/api/internal/config"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/gcpauth"
	"github.com/leoklaser/video-analyzer/api/internal/gcs"
	"github.com/leoklaser/video-analyzer/api/internal/handlers"
	"github.com/leoklaser/video-analyzer/api/internal/jobs"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	credsPath := "/tmp/gcp-sa.json"
	if err := gcpauth.WriteCredsFile(cfg.GoogleAppCredsJSON, credsPath); err != nil {
		slog.Error("gcpauth", "err", err)
		os.Exit(1)
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := db.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db open", "err", err)
		os.Exit(1)
	}
	defer d.Close()
	if err := d.Init(rootCtx); err != nil {
		slog.Error("db init", "err", err)
		os.Exit(1)
	}

	gcsClient, err := gcs.New(rootCtx, cfg.GCSBucket)
	if err != nil {
		slog.Error("gcs", "err", err)
		os.Exit(1)
	}
	defer gcsClient.Close()

	scriptPath := os.Getenv("ANALYZE_VIDEO_SCRIPT")
	if scriptPath == "" {
		abs, _ := filepath.Abs("tools/analyze-video.js")
		scriptPath = abs
	}
	runner := &jobs.Runner{
		DB:       d,
		Analyzer: analyzer.New(scriptPath),
		Claude:   claude.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicModel, ""),
		GCS:      gcsClient,
	}

	go jobs.StartWatchdog(rootCtx, d)

	mux := handlers.NewRouter(handlers.Deps{
		Uploads:        &handlers.UploadsHandler{GCS: gcsClient},
		Analyze:        &handlers.AnalyzeHandler{DB: d, Bucket: cfg.GCSBucket, Runner: runner},
		Analyses:       &handlers.AnalysesHandler{DB: d},
		Auth:           &handlers.AuthHandler{DB: d, JWTSecret: cfg.JWTSecret},
		AllowedOrigins: cfg.AllowedOrigins,
		JWTSecret:      cfg.JWTSecret,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
	}

	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			cancel()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")

	shutdownCtx, c2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer c2()
	_ = srv.Shutdown(shutdownCtx)
	cancel()
}
