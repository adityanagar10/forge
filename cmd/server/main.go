package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"forge/internal/api"
	"forge/internal/job"
	"forge/internal/store"
	"forge/internal/worker"
)

func main() {
	ctx := context.Background()

	// Connect to database
	connString := getEnv("DATABASE_URL", "postgres://jobqueue:jobqueue@localhost:5432/jobqueue")
	db, err := store.NewPostgresStore(ctx, connString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create worker pool
	pool := worker.New(db, worker.Config{
		Concurrency:  5,
		PollInterval: time.Second,
	})

	// Register job handlers
	pool.Register("email", createHandler("email"))
	pool.Register("webhook", createHandler("webhook"))
	pool.Register("report", createHandler("report"))
	pool.Register("example", createHandler("example"))

	// Start workers
	pool.Start()

	// Create API server
	srv := api.New(db)
	httpServer := &http.Server{
		Addr:    getEnv("PORT", ":8080"),
		Handler: srv,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down...")

		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)

		pool.Stop()
	}()

	log.Printf("Server starting on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

type jobPayload struct {
	ID         int    `json:"id"`
	ShouldFail bool   `json:"shouldFail"`
	Timestamp  int64  `json:"timestamp"`
	Message    string `json:"message"`
}

func createHandler(jobType string) worker.Handler {
	return func(ctx context.Context, j *job.Job) error {
		var payload jobPayload
		if err := json.Unmarshal(j.Payload, &payload); err != nil {
			log.Printf("[%s] Failed to parse payload: %v", jobType, err)
			return err
		}

		// Simulate some processing time
		processingTime := time.Duration(100+rand.Intn(400)) * time.Millisecond
		time.Sleep(processingTime)

		log.Printf("[%s] Processing job %s: %s (took %v)", jobType, j.ID[:8], payload.Message, processingTime)

		if payload.ShouldFail {
			return errors.New("simulated failure: job configured to fail")
		}

		return nil
	}
}
