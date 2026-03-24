// Package main is the entry point for the background worker service.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ismobaga/synt/internal/content"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/jobs"
	"github.com/ismobaga/synt/internal/media"
	"github.com/ismobaga/synt/internal/moderation"
	"github.com/ismobaga/synt/internal/music"
	"github.com/ismobaga/synt/internal/render"
	"github.com/ismobaga/synt/internal/subtitle"
	"github.com/ismobaga/synt/internal/voice"
	"github.com/ismobaga/synt/pkg/ffmpeg"
	"github.com/ismobaga/synt/pkg/llm"
	"github.com/ismobaga/synt/pkg/tts"
)

func main() {
	databaseURL := getEnv("DATABASE_URL", "postgres://synt:synt@localhost:5432/synt?sslmode=disable")

	database, err := db.New(databaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	// Wire up services using stub implementations.
	// Replace with real providers in production.
	llmClient := llm.NewStubClient()
	ttsClient := tts.NewStubClient()
	ffmpegRunner := ffmpeg.NewLocalRunner()

	contentSvc := content.New(llmClient)
	mediaSvc := media.New()
	voiceSvc := voice.New(ttsClient)
	subtitleSvc := subtitle.New()
	musicSvc := music.New(music.NewDefaultLibrary())
	renderSvc := render.New(database, ffmpegRunner)
	moderationSvc := moderation.New()

	worker := jobs.New(
		database,
		contentSvc,
		mediaSvc,
		voiceSvc,
		subtitleSvc,
		musicSvc,
		renderSvc,
		moderationSvc,
		jobs.Config{PollInterval: 5 * time.Second},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("worker: received shutdown signal")
		cancel()
	}()

	worker.Run(ctx)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
