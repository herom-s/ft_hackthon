package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ft_hackthon/internal/config"
	"gopkg.in/yaml.v3"
)

// SubmitManager handles the grading submission and polling workflow
type SubmitManager struct {
	apiClient *APIClient
	ui        *TerminalUI
}

// NewSubmitManager creates a new submit manager
func NewSubmitManager(apiClient *APIClient) *SubmitManager {
	return &SubmitManager{
		apiClient: apiClient,
		ui:        NewTerminalUI(),
	}
}

// SubmitGradeJob handles the complete grading submission flow:
// 1. Copies project files to the workspace repo
// 2. Pushes to Gitea
// 3. Submits the commit SHA to the API
// 4. Polls for results
func (sm *SubmitManager) SubmitGradeJob() error {
	ws, err := WorkspaceDir()
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}

	if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
		return fmt.Errorf("workspace not found at %s - please run 'login' first", ws)
	}

	fmt.Println("Pushing workspace code to Gitea...")
	commitSHA, err := PushToGitea(ws)
	if err != nil {
		return fmt.Errorf("failed to push code to Gitea: %w", err)
	}

	fmt.Printf("Pushed commit: %s\n", commitSHA[:12])

	suite := readSuiteConfig(ws)

	fmt.Println("Submitting to grader...")
	submitResp, err := sm.apiClient.Submit(commitSHA, suite)
	if err != nil {
		return fmt.Errorf("submission failed: %w", err)
	}

	OKf("Submitted %s", Dim("(job "+submitResp.JobID+")"))
	return sm.PollStatus(submitResp.JobID)
}

func readSuiteConfig(dir string) string {
	cfgPath := filepath.Join(dir, "ft_hackthon.yml")
	f, err := os.Open(cfgPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	var cfg struct {
		Suite string `yaml:"suite"`
	}
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return ""
	}
	return cfg.Suite
}

// PromptSuiteSelection asks what hackathon the user is participating in
// and returns the suite name. Does not create any files.
func (sm *SubmitManager) PromptSuiteSelection() (string, error) {
	suitesResp, err := sm.apiClient.ListSuites()
	if err != nil {
		return "", fmt.Errorf("failed to fetch hackathons: %w", err)
	}

	if len(suitesResp.Suites) == 0 {
		fmt.Println("   No hackathons available.")
		return "", nil
	}

	fmt.Println()
	fmt.Println("Available hackathons:")
	for _, s := range suitesResp.Suites {
		status := "✓"
		if !s.Active {
			status = "✗"
		}
		window := ""
		if s.StartsAt != "" || s.EndsAt != "" {
			window = fmt.Sprintf("  [%s – %s]", s.StartsAt, s.EndsAt)
		}
		fmt.Printf("  %s %s%s\n", status, s.Name, window)
		if s.Message != "" {
			fmt.Printf("     (%s)\n", s.Message)
		}
	}
	fmt.Println()

	for {
		fmt.Print("What hackathon are you participating in? ")
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		var matched *SuiteInfo
		for _, s := range suitesResp.Suites {
			if strings.EqualFold(input, s.Name) {
				matched = &s
				input = s.Name
				break
			}
		}

		if matched == nil {
			Failf("Unknown hackathon %q.", input)
			continue
		}

		if !matched.Active {
			Failf("%q is not accepting submissions: %s", input, matched.Message)
			continue
		}

		OKf("Registered for %q hackathon. Test suite configured.", input)

		// Download challenge subjects to ~/ft_hackthon/<hackathon>/
		sm.downloadChallengeSubjects(input)

		return input, nil
	}
}

func (sm *SubmitManager) downloadChallengeSubjects(suite string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	subjectsDir := filepath.Join(home, "ft_hackthon", suite)
	os.MkdirAll(subjectsDir, 0755)

	chResp, err := sm.apiClient.GetChallenges(suite)
	if err != nil {
		Warnf("Could not fetch challenge subjects: %v", err)
		return
	}

	if len(chResp.Challenges) == 0 {
		return
	}

	for _, ch := range chResp.Challenges {
		subjectPath := filepath.Join(subjectsDir, ch.Name+".txt")
		if err := os.WriteFile(subjectPath, []byte(ch.Subject), 0644); err != nil {
			Warnf("Could not write %s: %v", subjectPath, err)
			continue
		}
	}
	fmt.Printf("Challenge subjects saved to ~/ft_hackthon/%s/\n", suite)
}

// PollStatus waits for a job to finish and prints its result. Updates stream
// over WebSocket when the server allows it, with HTTP polling as the
// fallback. In the interactive shell a single live line shows progress;
// otherwise one line is printed per status change.
func (sm *SubmitManager) PollStatus(jobID string) error {
	live := newLiveLine()
	defer live.Stop()
	if !live.active {
		fmt.Println("Waiting for grading to complete...")
	}
	live.Set("Connecting to the grader…")

	done := make(chan struct{})
	defer close(done)
	var updates <-chan *StatusResponse
	wsc := sm.apiClient.newWSClient()
	if ch, err := wsc.Watch(jobID, done); err == nil {
		updates = ch
		defer wsc.Close()
	}

	timeout := time.Duration(maxPollingAttempts*pollingInterval) * time.Second
	deadline := time.After(timeout)
	poll := time.NewTicker(time.Duration(pollingInterval) * time.Second)
	defer poll.Stop()
	pollNow := make(chan struct{}, 1)
	if updates == nil {
		pollNow <- struct{}{}
	}

	lastStatus := ""
	unreachable := false
	handle := func(s *StatusResponse) (bool, error) {
		if s.Status != lastStatus {
			lastStatus = s.Status
			live.Set(getStatusMessage(s.Status))
			if !live.active && !isTerminalStatus(s.Status) {
				sm.ui.PrintStatusUpdate(s)
			}
		}
		switch s.Status {
		case "completed":
			live.Stop()
			if s.Result != nil {
				sm.ui.PrintGradeResult(s.Result)
			}
			return true, nil
		case "failed", "error":
			live.Stop()
			Failf("Grading failed: %s", s.Message)
			return true, reportedError{fmt.Errorf("grading job failed with status: %s", s.Status)}
		}
		return false, nil
	}
	pollOnce := func() (bool, error) {
		s, err := sm.apiClient.GetStatus(jobID)
		if err != nil {
			if !unreachable {
				unreachable = true
				live.Set(Yellow("Lost contact with the server, retrying…"))
				if !live.active {
					Warnf("Can't fetch status, retrying: %v", err)
				}
			}
			return false, nil
		}
		if unreachable {
			unreachable = false
			live.Set(getStatusMessage(s.Status))
		}
		return handle(s)
	}

	for {
		var pollC <-chan time.Time
		if updates == nil {
			pollC = poll.C
		}
		var finished bool
		var err error
		select {
		case s, ok := <-updates:
			if !ok {
				updates = nil
				pollNow <- struct{}{}
				continue
			}
			finished, err = handle(s)
		case <-pollNow:
			finished, err = pollOnce()
		case <-pollC:
			finished, err = pollOnce()
		case <-deadline:
			live.Stop()
			Failf("Gave up waiting after %s; the job is still queued or running. Check later with 'status %s'.", timeout, jobID)
			return reportedError{fmt.Errorf("timed out waiting for job %s", jobID)}
		}
		if finished {
			return err
		}
	}
}

func (sm *SubmitManager) HasGiteaConfig() bool {
	cfg, err := config.LoadConfig()
	if err != nil {
		return false
	}
	return cfg.GiteaCloneURL != "" && cfg.GiteaToken != ""
}
