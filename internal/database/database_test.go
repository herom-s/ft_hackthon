package database

import (
	"fmt"
	"testing"
	"time"
)

func TestNewInMemoryDB(t *testing.T) {
	db := NewInMemoryDB()
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	if db.users == nil {
		t.Error("expected users map to be initialized")
	}
	if db.jobs == nil {
		t.Error("expected jobs map to be initialized")
	}
}

func TestPing(t *testing.T) {
	db := NewInMemoryDB()
	if err := db.Ping(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestClose(t *testing.T) {
	db := NewInMemoryDB()
	if err := db.Close(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCreateUser(t *testing.T) {
	db := NewInMemoryDB()

	t.Run("success", func(t *testing.T) {
		user := &User{ID: "u1", Username: "alice", Email: "alice@test.com", Password: "secret"}
		err := db.CreateUser(user)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if user.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
		if user.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("empty id", func(t *testing.T) {
		err := db.CreateUser(&User{Username: "bob"})
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		err := db.CreateUser(&User{ID: "u1", Username: "alice2"})
		if err == nil {
			t.Fatal("expected error for duplicate ID")
		}
	})
}

func TestGetUser(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})

	t.Run("existing user", func(t *testing.T) {
		user, err := db.GetUser("u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if user.Username != "alice" {
			t.Errorf("expected alice, got %s", user.Username)
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		_, err := db.GetUser("u999")
		if err == nil {
			t.Fatal("expected error for non-existing user")
		}
	})
}

func TestGetUserByUsername(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})

	t.Run("existing username", func(t *testing.T) {
		user, err := db.GetUserByUsername("alice")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if user.ID != "u1" {
			t.Errorf("expected u1, got %s", user.ID)
		}
	})

	t.Run("non-existing username", func(t *testing.T) {
		_, err := db.GetUserByUsername("bob")
		if err == nil {
			t.Fatal("expected error for non-existing username")
		}
	})
}

func TestUpdateUser(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})

	t.Run("existing user", func(t *testing.T) {
		original := db.users["u1"].UpdatedAt
		time.Sleep(time.Millisecond)
		user := &User{ID: "u1", Username: "alice_updated"}
		err := db.UpdateUser(user)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if user.UpdatedAt.Equal(original) {
			t.Error("expected UpdatedAt to change")
		}
		if db.users["u1"].Username != "alice_updated" {
			t.Errorf("expected alice_updated, got %s", db.users["u1"].Username)
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		err := db.UpdateUser(&User{ID: "u999"})
		if err == nil {
			t.Fatal("expected error for non-existing user")
		}
	})
}

func TestDeleteUser(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})

	t.Run("existing user", func(t *testing.T) {
		err := db.DeleteUser("u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		_, err = db.GetUser("u1")
		if err == nil {
			t.Fatal("expected user to be deleted")
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		err := db.DeleteUser("u999")
		if err == nil {
			t.Fatal("expected error for non-existing user")
		}
	})
}

func TestCreateJob(t *testing.T) {
	db := NewInMemoryDB()

	t.Run("success", func(t *testing.T) {
		job := &Job{ID: "j1", UserID: "u1", CommitSHA: "abc123"}
		err := db.CreateJob(job)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if job.Status != JobStatusQueued {
			t.Errorf("expected queued status, got %s", job.Status)
		}
		if job.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("empty id", func(t *testing.T) {
		err := db.CreateJob(&Job{UserID: "u1"})
		if err == nil {
			t.Fatal("expected error for empty job ID")
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		err := db.CreateJob(&Job{ID: "j1"})
		if err == nil {
			t.Fatal("expected error for duplicate job ID")
		}
	})
}

func TestGetJob(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})

	t.Run("existing job", func(t *testing.T) {
		job, err := db.GetJob("j1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if job.ID != "j1" {
			t.Errorf("expected j1, got %s", job.ID)
		}
	})

	t.Run("non-existing job", func(t *testing.T) {
		_, err := db.GetJob("j999")
		if err == nil {
			t.Fatal("expected error for non-existing job")
		}
	})
}

func TestGetJobsByUser(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})
	db.CreateJob(&Job{ID: "j2", UserID: "u1"})
	db.CreateJob(&Job{ID: "j3", UserID: "u2"})

	jobs, err := db.GetJobsByUser("u1")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestUpdateJob(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})

	t.Run("existing job", func(t *testing.T) {
		original := db.jobs["j1"].UpdatedAt
		time.Sleep(time.Millisecond)
		job := &Job{ID: "j1", Status: JobStatusProcessing}
		err := db.UpdateJob(job)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if job.UpdatedAt.Equal(original) {
			t.Error("expected UpdatedAt to change")
		}
	})

	t.Run("non-existing job", func(t *testing.T) {
		err := db.UpdateJob(&Job{ID: "j999"})
		if err == nil {
			t.Fatal("expected error for non-existing job")
		}
	})
}

func TestGetPendingJobs(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"}) // queued (default)
	db.mu.Lock()
	db.jobs["j2"] = &Job{ID: "j2", UserID: "u1", Status: JobStatusProcessing}
	db.jobs["j3"] = &Job{ID: "j3", UserID: "u1", Status: JobStatusCompleted}
	db.jobs["j4"] = &Job{ID: "j4", UserID: "u1", Status: JobStatusFailed}
	db.mu.Unlock()
	db.CreateJob(&Job{ID: "j5", UserID: "u2"}) // queued (default)

	t.Run("returns only pending jobs", func(t *testing.T) {
		jobs, err := db.GetPendingJobs(10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 3 {
			t.Errorf("expected 3 pending jobs, got %d", len(jobs))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		jobs, err := db.GetPendingJobs(2)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) > 2 {
			t.Errorf("expected at most 2 jobs, got %d", len(jobs))
		}
	})
}

func TestSaveResult(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})
	r := &Result{ParserSuccess: true, BenchmarkMs: 100, FinalScore: 90}

	t.Run("success", func(t *testing.T) {
		err := db.SaveResult("j1", r)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		job, _ := db.GetJob("j1")
		if job.Status != JobStatusCompleted {
			t.Errorf("expected completed status, got %s", job.Status)
		}
		if job.Result == nil {
			t.Fatal("expected result to be set")
		}
		if job.Result.FinalScore != 90 {
			t.Errorf("expected 90, got %d", job.Result.FinalScore)
		}
	})

	t.Run("non-existing job", func(t *testing.T) {
		err := db.SaveResult("j999", r)
		if err == nil {
			t.Fatal("expected error for non-existing job")
		}
	})
}

func TestGetResult(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})

	t.Run("no result yet", func(t *testing.T) {
		_, err := db.GetResult("j1")
		if err == nil {
			t.Fatal("expected error when no result available")
		}
	})

	t.Run("non-existing job", func(t *testing.T) {
		_, err := db.GetResult("j999")
		if err == nil {
			t.Fatal("expected error for non-existing job")
		}
	})

	t.Run("existing result", func(t *testing.T) {
		db.SaveResult("j1", &Result{ParserSuccess: true, BenchmarkMs: 100, FinalScore: 90})
		result, err := db.GetResult("j1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if result.FinalScore != 90 {
			t.Errorf("expected 90, got %d", result.FinalScore)
		}
	})
}

func TestCreateToken(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})

	t.Run("success", func(t *testing.T) {
		err := db.CreateToken("token1", "u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		userID, exists := db.tokens["token1"]
		if !exists {
			t.Fatal("expected token to be stored")
		}
		if userID != "u1" {
			t.Errorf("expected u1, got %s", userID)
		}
	})

	t.Run("duplicate token overwrites", func(t *testing.T) {
		err := db.CreateToken("token1", "u2")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		userID, _ := db.GetUserIDByToken("token1")
		if userID != "u2" {
			t.Errorf("expected u2, got %s", userID)
		}
	})

	t.Run("accepts non-existing user", func(t *testing.T) {
		err := db.CreateToken("token2", "nonexistent")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
}

func TestGetUserIDByToken(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateToken("token1", "u1")

	t.Run("existing token", func(t *testing.T) {
		userID, err := db.GetUserIDByToken("token1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if userID != "u1" {
			t.Errorf("expected u1, got %s", userID)
		}
	})

	t.Run("non-existing token", func(t *testing.T) {
		_, err := db.GetUserIDByToken("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existing token")
		}
	})
}

func TestDeleteToken(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateToken("token1", "u1")

	t.Run("existing token", func(t *testing.T) {
		err := db.DeleteToken("token1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		_, err = db.GetUserIDByToken("token1")
		if err == nil {
			t.Fatal("expected token to be deleted")
		}
	})

	t.Run("non-existing token", func(t *testing.T) {
		err := db.DeleteToken("nonexistent")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
}

func TestUpdateRating(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice", Rating: DefaultRating})

	t.Run("existing user", func(t *testing.T) {
		original := db.users["u1"].UpdatedAt
		time.Sleep(time.Millisecond)
		err := db.UpdateRating("u1", 1500)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if db.users["u1"].Rating != 1500 {
			t.Errorf("expected 1500, got %d", db.users["u1"].Rating)
		}
		if db.users["u1"].UpdatedAt.Equal(original) {
			t.Error("expected UpdatedAt to change")
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		err := db.UpdateRating("u999", 1500)
		if err == nil {
			t.Fatal("expected error for non-existing user")
		}
	})
}

func TestGetLeaderboard(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})
	db.CreateUser(&User{ID: "u2", Username: "bob"})
	db.CreateUser(&User{ID: "u3", Username: "charlie"})

	// Completed jobs with results
	db.CreateJob(&Job{ID: "j1", UserID: "u1", Suite: "suite1"})
	db.SaveResult("j1", &Result{FinalScore: 90, BenchmarkMs: 100})

	db.CreateJob(&Job{ID: "j2", UserID: "u2", Suite: "suite1"})
	db.SaveResult("j2", &Result{FinalScore: 80, BenchmarkMs: 200})

	// Different suite
	db.CreateJob(&Job{ID: "j3", UserID: "u3", Suite: "other"})
	db.SaveResult("j3", &Result{FinalScore: 95, BenchmarkMs: 50})

	// Non-completed job (excluded)
	db.mu.Lock()
	db.jobs["j4"] = &Job{ID: "j4", UserID: "u1", Suite: "suite1", Status: JobStatusFailed}
	db.mu.Unlock()

	t.Run("returns entries sorted by score descending", func(t *testing.T) {
		entries, err := db.GetLeaderboard("suite1", 10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].Username != "alice" || entries[0].Score != 90 {
			t.Errorf("expected alice with 90, got %s with %d", entries[0].Username, entries[0].Score)
		}
		if entries[1].Username != "bob" || entries[1].Score != 80 {
			t.Errorf("expected bob with 80, got %s with %d", entries[1].Username, entries[1].Score)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		entries, err := db.GetLeaderboard("suite1", 1)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}
	})

	t.Run("empty for suite with no entries", func(t *testing.T) {
		entries, err := db.GetLeaderboard("empty", 10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("picks best score per user", func(t *testing.T) {
		db.CreateJob(&Job{ID: "j5", UserID: "u1", Suite: "bestscore"})
		db.SaveResult("j5", &Result{FinalScore: 70, BenchmarkMs: 150})
		db.CreateJob(&Job{ID: "j6", UserID: "u1", Suite: "bestscore"})
		db.SaveResult("j6", &Result{FinalScore: 95, BenchmarkMs: 80})

		entries, err := db.GetLeaderboard("bestscore", 10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Score != 95 {
			t.Errorf("expected best score 95, got %d", entries[0].Score)
		}
	})
}

func TestGetSuiteScores(t *testing.T) {
	db := NewInMemoryDB()

	db.CreateJob(&Job{ID: "j1", UserID: "u1", Suite: "suite1"})
	db.SaveResult("j1", &Result{FinalScore: 90})

	db.CreateJob(&Job{ID: "j2", UserID: "u2", Suite: "suite1"})
	db.SaveResult("j2", &Result{FinalScore: 80})

	// Different suite
	db.CreateJob(&Job{ID: "j3", UserID: "u3", Suite: "other"})
	db.SaveResult("j3", &Result{FinalScore: 95})

	// Non-completed job with result (SaveResult forces completed)
	db.CreateJob(&Job{ID: "j4", UserID: "u1", Suite: "suite1"})

	t.Run("returns all scores for suite", func(t *testing.T) {
		scores, err := db.GetSuiteScores("suite1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(scores) != 2 {
			t.Fatalf("expected 2 scores, got %d", len(scores))
		}
	})

	t.Run("empty for suite with no completed jobs", func(t *testing.T) {
		scores, err := db.GetSuiteScores("nonexistent")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(scores) != 0 {
			t.Errorf("expected 0 scores, got %d", len(scores))
		}
	})
}

func TestCheckPlagiarism(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateUser(&User{ID: "u1", Username: "alice"})
	db.CreateUser(&User{ID: "u2", Username: "bob"})
	db.CreateUser(&User{ID: "u3", Username: "charlie"})

	db.CreateJob(&Job{ID: "j1", UserID: "u1", Suite: "suite1"})
	db.SaveResult("j1", &Result{CodeChecksum: "cs1", FinalScore: 90})

	db.CreateJob(&Job{ID: "j2", UserID: "u2", Suite: "suite1"})
	db.SaveResult("j2", &Result{CodeChecksum: "cs1", FinalScore: 80})

	db.CreateJob(&Job{ID: "j3", UserID: "u3", Suite: "suite1"})
	db.SaveResult("j3", &Result{CodeChecksum: "cs2", FinalScore: 70})

	t.Run("detects duplicate checksums", func(t *testing.T) {
		groups, err := db.CheckPlagiarism("suite1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(groups) != 1 {
			t.Fatalf("expected 1 plagiarism group, got %d", len(groups))
		}
		if groups[0].Checksum != "cs1" {
			t.Errorf("expected cs1, got %s", groups[0].Checksum)
		}
		if groups[0].UserCount != 2 {
			t.Errorf("expected 2 users, got %d", groups[0].UserCount)
		}
	})

	t.Run("no plagiarism when all checksums unique", func(t *testing.T) {
		db.CreateJob(&Job{ID: "j4", UserID: "u1", Suite: "other"})
		db.SaveResult("j4", &Result{CodeChecksum: "unique1", FinalScore: 100})
		db.CreateJob(&Job{ID: "j5", UserID: "u2", Suite: "other"})
		db.SaveResult("j5", &Result{CodeChecksum: "unique2", FinalScore: 90})

		groups, err := db.CheckPlagiarism("other")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(groups) != 0 {
			t.Errorf("expected 0 plagiarism groups, got %d", len(groups))
		}
	})

	t.Run("ignores jobs without checksum", func(t *testing.T) {
		db.CreateJob(&Job{ID: "j6", UserID: "u3", Suite: "nocsuite"})
		db.SaveResult("j6", &Result{FinalScore: 50})

		groups, err := db.CheckPlagiarism("nocsuite")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(groups) != 0 {
			t.Errorf("expected 0 plagiarism groups, got %d", len(groups))
		}
	})
}

func TestConcurrency(t *testing.T) {
	db := NewInMemoryDB()
	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(n int) {
			id := fmt.Sprintf("u%d", n)
			db.CreateUser(&User{ID: id, Username: id})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	for i := 0; i < 10; i++ {
		_, err := db.GetUser(fmt.Sprintf("u%d", i))
		if err != nil {
			t.Errorf("expected user u%d to exist", i)
		}
	}
}

func TestClaimJobs(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})
	db.CreateJob(&Job{ID: "j2", UserID: "u2"})

	t.Run("claims queued jobs", func(t *testing.T) {
		jobs, err := db.ClaimJobs("worker-1", 10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("expected 2 jobs, got %d", len(jobs))
		}
		for _, j := range jobs {
			if j.Status != JobStatusProcessing {
				t.Errorf("expected processing, got %s", j.Status)
			}
		}
	})

	t.Run("no more queued jobs after claiming", func(t *testing.T) {
		jobs, err := db.ClaimJobs("worker-2", 10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 0 {
			t.Errorf("expected 0 jobs, got %d", len(jobs))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		db2 := NewInMemoryDB()
		db2.CreateJob(&Job{ID: "k1", UserID: "u1"})
		db2.CreateJob(&Job{ID: "k2", UserID: "u2"})
		db2.CreateJob(&Job{ID: "k3", UserID: "u3"})

		jobs, err := db2.ClaimJobs("worker-3", 2)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("expected 2 jobs, got %d", len(jobs))
		}
		// Remaining queued jobs still claimable
		remaining, _ := db2.ClaimJobs("worker-4", 10)
		if len(remaining) != 1 {
			t.Errorf("expected 1 remaining job, got %d", len(remaining))
		}
	})
}

func TestReleaseStuckJobs(t *testing.T) {
	db := NewInMemoryDB()
	db.CreateJob(&Job{ID: "j1", UserID: "u1"})
	db.CreateJob(&Job{ID: "j2", UserID: "u1"})
	db.ClaimJobs("worker", 10)

	t.Run("releases jobs past timeout", func(t *testing.T) {
		// Set updated_at far in the past to simulate stuck jobs
		db.mu.Lock()
		db.jobs["j1"].UpdatedAt = time.Now().Add(-30 * time.Minute)
		db.jobs["j2"].UpdatedAt = time.Now().Add(-5 * time.Minute)
		db.mu.Unlock()

		count, err := db.ReleaseStuckJobs(10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 stuck job released, got %d", count)
		}
		// j1 should be released, j2 should still be processing
		j1, _ := db.GetJob("j1")
		j2, _ := db.GetJob("j2")
		if j1.Status != JobStatusQueued {
			t.Errorf("expected j1 to be queued, got %s", j1.Status)
		}
		if j2.Status != JobStatusProcessing {
			t.Errorf("expected j2 to remain processing, got %s", j2.Status)
		}
	})
}
