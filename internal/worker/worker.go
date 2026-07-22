package worker

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/grader"
)

func init() {
	sp := os.Getenv("TESTSUITES_PATH")
	if sp == "" {
		sp = "/var/ft_hackthon/testsuites"
	}
	if _, err := os.Stat(sp); err == nil {
		grader.SetSuitesPath(sp)
	}
}

var workerID string

func init() {
	workerID = os.Getenv("WORKER_ID")
	if workerID == "" {
		host, err := os.Hostname()
		if err != nil {
			workerID = "unknown"
		} else {
			workerID = host
		}
	}
}

const (
	defaultJobTimeout   = 5 * time.Minute
	maxRetries          = 3
	circuitThreshold    = 5
	circuitResetTimeout = 30 * time.Second
)

// Worker processes grading jobs
type Worker struct {
	db              database.Database
	jobChannel      chan *database.Job
	done            chan bool
	pollInterval    time.Duration
	releaseInterval time.Duration
	claimLimit      int
	jobTimeout      time.Duration
	giteaCB         *CircuitBreaker
}

// NewWorker creates a new worker
func NewWorker(db database.Database) *Worker {
	return &Worker{
		db:              db,
		jobChannel:      make(chan *database.Job, 10),
		done:            make(chan bool),
		pollInterval:    5 * time.Second,
		releaseInterval: 1 * time.Minute,
		claimLimit:      5,
		jobTimeout:      defaultJobTimeout,
		giteaCB:         NewCircuitBreaker(circuitThreshold, circuitResetTimeout),
	}
}

// Start begins processing jobs
func (w *Worker) Start() {
	log.Printf("Worker %s started - claiming jobs...", workerID)

	go func() {
		pollTicker := time.NewTicker(w.pollInterval)
		releaseTicker := time.NewTicker(w.releaseInterval)
		defer pollTicker.Stop()
		defer releaseTicker.Stop()

		for {
			select {
			case <-w.done:
				log.Println("Worker stopped")
				return
			case <-pollTicker.C:
				w.claimAndProcess()
			case <-releaseTicker.C:
				w.releaseStuckJobs()
			}
		}
	}()
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	w.done <- true
}

// claimAndProcess atomically claims pending jobs and processes them
func (w *Worker) claimAndProcess() {
	jobs, err := w.db.ClaimJobs(workerID, w.claimLimit)
	if err != nil {
		log.Printf("Error claiming jobs: %v", err)
		return
	}

	for _, job := range jobs {
		w.processJob(job)
	}
}

// releaseStuckJobs resets jobs that were claimed but never completed
func (w *Worker) releaseStuckJobs() {
	released, err := w.db.ReleaseStuckJobs(10)
	if err != nil {
		log.Printf("Error releasing stuck jobs: %v", err)
		return
	}
	if released > 0 {
		log.Printf("Released %d stuck jobs for re-claiming", released)
	}
}

// processJob processes a single grading job with retry and timeout
func (w *Worker) processJob(job *database.Job) {
	shortSHA := job.CommitSHA
	if len(shortSHA) > 12 {
		shortSHA = shortSHA[:12]
	}
	log.Printf("Processing job: %s (commit: %s)", job.ID, shortSHA)

	job.Status = database.JobStatusProcessing
	job.Message = "Cloning repository and running tests..."
	if err := w.db.UpdateJob(job); err != nil {
		log.Printf("Error updating job status: %v", err)
		return
	}

	var result *database.Result
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if !w.giteaCB.Allow() {
			log.Printf("Circuit breaker open for job %s, attempt %d/%d", job.ID, attempt, maxRetries)
			time.Sleep(5 * time.Second)
			continue
		}

		done := make(chan struct{})
		var gradeResult *database.Result
		go func() {
			gradeResult = w.gradeProject(job)
			close(done)
		}()

		select {
		case <-done:
			result = gradeResult
			if result.ParserSuccess || result.FinalScore > 0 {
				w.giteaCB.Success()
			} else if attempt < maxRetries {
				w.giteaCB.Failure()
				log.Printf("Job %s attempt %d failed, retrying...", job.ID, attempt)
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
		case <-time.After(w.jobTimeout):
			w.giteaCB.Failure()
			if attempt < maxRetries {
				log.Printf("Job %s timed out on attempt %d/%d, retrying...", job.ID, attempt, maxRetries)
				continue
			}
			result = &database.Result{
				ParserSuccess: false,
				Details:       "Job timed out after all retries",
			}
		}
		break
	}

	if result == nil {
		result = &database.Result{
			ParserSuccess: false,
			Details:       "All retries exhausted",
		}
	}

	if err := w.db.SaveResult(job.ID, result); err != nil {
		log.Printf("Error saving result: %v", err)
		job.Status = database.JobStatusError
		job.Message = "Error saving grading result"
		w.db.UpdateJob(job)
		return
	}

	if result.ParserSuccess {
		user, err := w.db.GetUser(job.UserID)
		if err == nil {
			currentRating := user.Rating
			if currentRating == 0 {
				currentRating = grader.DefaultEloRating
			}
			newRating := grader.ComputeNewRating(currentRating, result.FinalScore)
			if err := w.db.UpdateRating(job.UserID, newRating); err != nil {
				log.Printf("Error updating rating for user %s: %v", job.UserID, err)
			} else {
				log.Printf("Rating updated for user %s: %d -> %d", user.Username, currentRating, newRating)
			}
		}
	}

	log.Printf("Job completed: %s - Parser: %v, Score: %d",
		job.ID, result.ParserSuccess, result.FinalScore)
}

// gradeProject clones the repo from Gitea and runs the matching test suite
func (w *Worker) gradeProject(job *database.Job) *database.Result {
	// Create temp directory for cloning
	tmpDir, err := os.MkdirTemp("", "ft-hackthon-grade-*")
	if err != nil {
		return &database.Result{
			ParserSuccess: false,
			Details:       fmt.Sprintf("Failed to create temp dir: %v", err),
		}
	}
	defer os.RemoveAll(tmpDir)

	// Clone repo from Gitea
	cloneURL := job.GiteaCloneURL
	if cloneURL == "" {
		return &database.Result{
			ParserSuccess: false,
			Details:       "No Gitea clone URL available for this job",
		}
	}

	cloneDir := filepath.Join(tmpDir, "repo")
	cmd := exec.Command("git", "clone", cloneURL, cloneDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return &database.Result{
			ParserSuccess: false,
			Details:       fmt.Sprintf("Failed to clone repo: %v\n%s", err, output),
		}
	}

	// Checkout the specific commit
	checkoutCmd := exec.Command("git", "checkout", job.CommitSHA)
	checkoutCmd.Dir = cloneDir
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return &database.Result{
			ParserSuccess: false,
			Details:       fmt.Sprintf("Failed to checkout commit %s: %v\n%s", job.CommitSHA, err, output),
		}
	}

	r := grader.Grade(cloneDir, job.Suite)
	if r == nil {
		return &database.Result{
			ParserSuccess: false,
			Details:       "No matching test suite found",
		}
	}
	return ToDatabaseResult(r)
}

func ToDatabaseResult(r *grader.Result) *database.Result {
	if r == nil {
		return nil
	}
	dbResult := &database.Result{
		ParserSuccess: r.ParserSuccess,
		BenchmarkMs:   r.BenchmarkMs,
		FinalScore:    r.FinalScore,
		Details:       r.Details,
		CodeChecksum:  r.CodeChecksum,
	}
	for _, ch := range r.Challenges {
		dbResult.Challenges = append(dbResult.Challenges, database.ChallengeDetail{
			Name:        ch.Name,
			Title:       ch.Title,
			Passed:      ch.Passed,
			Points:      ch.Points,
			TestsRun:    ch.TestsRun,
			TestsPassed: ch.TestsPassed,
			BenchmarkMs: ch.BenchmarkMs,
			Details:     ch.Details,
		})
	}
	return dbResult
}
