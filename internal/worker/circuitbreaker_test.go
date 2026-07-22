package worker

import (
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts closed and allows", func(t *testing.T) {
		cb := NewCircuitBreaker(3, time.Minute)
		if !cb.Allow() {
			t.Error("expected circuit to allow initially")
		}
	})

	t.Run("opens after threshold failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3, time.Minute)
		cb.Failure()
		cb.Failure()
		cb.Failure()
		if cb.Allow() {
			t.Error("expected circuit to be open after 3 failures")
		}
	})

	t.Run("half-opens after reset timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 50*time.Millisecond)
		cb.Failure()
		cb.Failure()
		cb.Failure()
		cb.Allow() // should be blocked
		time.Sleep(60 * time.Millisecond)
		if !cb.Allow() {
			t.Error("expected circuit to half-open after reset timeout")
		}
	})

	t.Run("closes after success in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 50*time.Millisecond)
		cb.Failure()
		cb.Failure()
		cb.Failure()
		cb.Allow()
		time.Sleep(60 * time.Millisecond)
		cb.Allow() // half-open
		cb.Success()
		if !cb.Allow() {
			t.Error("expected circuit to close after success")
		}
	})

	t.Run("success resets failure count", func(t *testing.T) {
		cb := NewCircuitBreaker(3, time.Minute)
		cb.Failure()
		cb.Failure()
		cb.Success()
		cb.Failure()
		if !cb.Allow() {
			t.Error("expected circuit to still allow after 2 failures with reset")
		}
	})
}
