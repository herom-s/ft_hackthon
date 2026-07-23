package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/worker"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║   ft_hackthon Background Worker             ║")
	fmt.Println("║   Starting job processor...                ║")
	fmt.Println("╚════════════════════════════════════════════╝")

	// Initialize PostgreSQL (requires postgres container to be running)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required (e.g. postgres://user:pass@localhost:5432/ft_hackthon?sslmode=disable)")
	}

	log.Printf("Connecting to PostgreSQL...")
	db, err := database.NewPostgresDB(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")
	defer db.Close()

	// Create and start worker
	w := worker.NewWorker(db)
	w.Start()

	fmt.Println("\n+ Worker is running and listening for jobs...")
	fmt.Println("  Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	fmt.Println("\n\nShutting down worker...")
	w.Stop()
	fmt.Println("+ Worker stopped")
}
