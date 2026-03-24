// Package main is the entry point for the API gateway service.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ismobaga/synt/internal/api"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/jobs"
	"github.com/ismobaga/synt/internal/orchestrator"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := loadConfig()

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer redisClient.Close()

	queue := jobs.NewRedisQueue(redisClient)
	orch := orchestrator.New(database, queue)
	router := api.NewRouter(database, orch)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("api-gateway listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down api-gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

type config struct {
	Addr        string
	DatabaseURL string
	RedisAddr   string
}

func loadConfig() config {
	return config{
		Addr:        getEnv("ADDR", ":8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://synt:synt@localhost:5432/synt?sslmode=disable"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
