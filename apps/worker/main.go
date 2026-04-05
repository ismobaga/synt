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
	"github.com/ismobaga/synt/pkg/s3util"
	"github.com/ismobaga/synt/pkg/tts"
)

func main() {
	databaseURL := getEnv("DATABASE_URL", "postgres://synt:synt@localhost:5432/synt?sslmode=disable")

	database, err := db.New(databaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	// Wire up services using configurable provider implementations.
	llmClient, err := llm.NewFromEnv()
	if err != nil {
		log.Fatalf("configure llm client: %v", err)
	}
	ttsClient, err := tts.NewFromEnv()
	if err != nil {
		log.Fatalf("configure tts client: %v", err)
	}
	storageClient, err := s3util.NewFromEnv()
	if err != nil {
		log.Fatalf("configure storage client: %v", err)
	}
	ffmpegRunner := ffmpeg.NewLocalRunner()

	contentSvc := content.New(llmClient)
	var mediaProviders []media.Provider
	if pixabayProvider := media.NewPixabayProviderFromEnv(); pixabayProvider != nil {
		mediaProviders = append(mediaProviders, pixabayProvider)
		log.Println("media: Pixabay provider enabled")
	} else {
		log.Println("media: Pixabay provider not configured, using placeholders when needed")
	}
	mediaSvc := media.New(mediaProviders...)
	voiceSvc := voice.New(ttsClient, storageClient)
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
