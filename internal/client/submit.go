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

	fmt.Println("📤 Pushing workspace code to Gitea...")
	commitSHA, err := PushToGitea(ws)
	if err != nil {
		return fmt.Errorf("failed to push code to Gitea: %w", err)
	}

	fmt.Printf("📦 Pushed commit: %s\n", commitSHA[:12])

	suite := readSuiteConfig(ws)

	fmt.Println("📤 Submitting to grader...")
	submitResp, err := sm.apiClient.Submit(commitSHA, suite)
	if err != nil {
		return fmt.Errorf("submission failed: %w", err)
	}

	fmt.Printf("✓ Job ID: %s\n\n", submitResp.JobID)

	fmt.Println("⏳ Waiting for grading to complete...")
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
			fmt.Printf("❌ Unknown hackathon %q.\n", input)
			continue
		}

		if !matched.Active {
			fmt.Printf("❌ %q is not accepting submissions: %s\n", input, matched.Message)
			continue
		}

		fmt.Printf("✓ Registered for %q hackathon. Test suite configured.\n", input)

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
		fmt.Printf("⚠ Could not fetch challenge subjects: %v\n", err)
		return
	}

	if len(chResp.Challenges) == 0 {
		return
	}

	for _, ch := range chResp.Challenges {
		subjectPath := filepath.Join(subjectsDir, ch.Name+".txt")
		if err := os.WriteFile(subjectPath, []byte(ch.Subject), 0644); err != nil {
			fmt.Printf("⚠ Could not write %s: %v\n", subjectPath, err)
			continue
		}
	}
	fmt.Printf("📚 Challenge subjects saved to ~/ft_hackthon/%s/\n", suite)
}

// PollStatus continuously polls the API for job status
func (sm *SubmitManager) PollStatus(jobID string) error {
	ticker := time.NewTicker(time.Duration(pollingInterval) * time.Second)
	defer ticker.Stop()

	attempt := 0
	var lastStatus string

	for {
		select {
		case <-ticker.C:
			attempt++

			statusResp, err := sm.apiClient.GetStatus(jobID)
			if err != nil {
				fmt.Printf("⚠ Error fetching status: %v\n", err)
				if attempt > maxPollingAttempts {
					return fmt.Errorf("polling timeout: exceeded %d attempts", maxPollingAttempts)
				}
				continue
			}

			if statusResp.Status != lastStatus {
				lastStatus = statusResp.Status
				sm.ui.PrintStatusUpdate(statusResp)
			}

			if statusResp.Status == "completed" {
				if statusResp.Result != nil {
					fmt.Println()
					sm.ui.PrintGradeResult(statusResp.Result)
				}
				return nil
			}

			if statusResp.Status == "failed" || statusResp.Status == "error" {
				fmt.Printf("\n❌ Grading failed: %s\n", statusResp.Message)
				return fmt.Errorf("grading job failed with status: %s", statusResp.Status)
			}

			if attempt > maxPollingAttempts {
				return fmt.Errorf("polling timeout: exceeded %d attempts (%.0f minutes)",
					maxPollingAttempts, float64(maxPollingAttempts*pollingInterval)/60)
			}
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
