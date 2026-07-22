package main

import (
	"testing"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/worker"
)

func TestWorkerLifecycle(t *testing.T) {
	db := database.NewInMemoryDB()
	w := worker.NewWorker(db)

	w.Start()
	w.Stop()
}

func TestInMemoryDBCreation(t *testing.T) {
	db := database.NewInMemoryDB()
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	if err := db.Ping(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	db.Close()
}
