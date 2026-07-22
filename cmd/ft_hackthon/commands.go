package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the ft_hackthon server",
	Long:  `Log in to the ft_hackthon server using your credentials. Your token will be securely stored locally.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()
		authManager := client.NewAuthManager(apiClient)

		fmt.Println("Welcome to ft_hackthon!")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")

		resp, err := authManager.Login()
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)
		if resp.GiteaCloneURL != "" {
			ws, err := ensureGiteaRepo(resp.GiteaCloneURL)
			if err != nil {
				fmt.Printf("⚠ Failed to setup workspace: %v\n", err)
			} else {
				sm := client.NewSubmitManager(apiClient)
				promptSuiteSelection(sm, ws)
			}
		}
		return nil
	},
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create a new account on the ft_hackthon server",
	Long:  `Register a new account with the ft_hackthon server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()
		authManager := client.NewAuthManager(apiClient)

		fmt.Println("Register New Account")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")

		resp, err := authManager.Register()
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)
		if resp.GiteaCloneURL != "" {
			ws, err := ensureGiteaRepo(resp.GiteaCloneURL)
			if err != nil {
				fmt.Printf("⚠ Failed to setup workspace: %v\n", err)
			} else {
				sm := client.NewSubmitManager(apiClient)
				promptSuiteSelection(sm, ws)
			}
		}
		return nil
	},
}

var grademeCmd = &cobra.Command{
	Use:   "grademe",
	Short: "Submit your current project for grading",
	Long: `Submit your current project for grading. The CLI will:
1. Copy project files to the workspace and push to Gitea
2. Submit the commit SHA to the grader
3. Poll for results in real-time`,
	RunE: func(cmd *cobra.Command, args []string) error {
		isAuth, err := config.IsAuthenticated()
		if err != nil {
			return fmt.Errorf("authentication check failed: %w", err)
		}

		if !isAuth {
			return fmt.Errorf("not authenticated: please run 'ft_hackthon login' first")
		}

		apiClient := newAPIClient()
		submitManager := client.NewSubmitManager(apiClient)

		if err := submitManager.SubmitGradeJob(); err != nil {
			return fmt.Errorf("grading submission failed: %w", err)
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [job_id]",
	Short: "List your jobs or check a specific job status",
	Long:  `List all grading jobs assigned to you, or check the status of a specific job by its ID.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		isAuth, err := config.IsAuthenticated()
		if err != nil {
			return fmt.Errorf("authentication check failed: %w", err)
		}

		if !isAuth {
			return fmt.Errorf("not authenticated: please run 'ft_hackthon login' first")
		}

		apiClient := newAPIClient()

		if len(args) == 0 {
			jobs, err := apiClient.ListJobs()
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}
			if len(jobs.Jobs) == 0 {
				fmt.Println("No jobs assigned to you.")
				return nil
			}
			fmt.Println()
			fmt.Println("Your jobs:")
			for _, j := range jobs.Jobs {
				emoji := "⏳"
				switch j.Status {
				case "completed":
					emoji = "✓"
				case "failed", "error":
					emoji = "❌"
				case "processing":
					emoji = "⚙"
				}
				fmt.Printf("  %s %s  [%s]  %s\n", emoji, j.JobID, j.Status, j.Message)
			}
			fmt.Println()
			return nil
		}

		jobID := args[0]
		ui := client.NewTerminalUI()

		fmt.Printf("Checking status for job: %s\n", jobID)

		statusResp, err := apiClient.GetStatus(jobID)
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		ui.PrintStatusUpdate(statusResp)

		if statusResp.Result != nil {
			ui.PrintGradeResult(statusResp.Result)
		}

		return nil
	},
}

var leaderboardCmd = &cobra.Command{
	Use:   "leaderboard [hackathon]",
	Short: "Show top scorers for a hackathon",
	Long:  `Display the leaderboard showing top scorers for the specified hackathon.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()

		hackathon := ""
		if len(args) > 0 {
			hackathon = args[0]
		} else {
			hackathon = readSuiteConfig(".")
		}
		if hackathon == "" {
			return fmt.Errorf("no hackathon specified: provide a name or run from a directory with ft_hackthon.yml")
		}

		lb, err := apiClient.GetLeaderboard(hackathon)
		if err != nil {
			return fmt.Errorf("failed to get leaderboard: %w", err)
		}

		if len(lb.Entries) == 0 {
			fmt.Printf("No entries yet for %s\n", hackathon)
			return nil
		}

		fmt.Println()
		fmt.Printf("🏆 Leaderboard - %s\n", hackathon)
		fmt.Println(strings.Repeat("─", 72))
		fmt.Printf("%-4s %-20s %-8s %-8s %s\n", "Rank", "User", "Score", "Rating", "Benchmark")
		fmt.Println(strings.Repeat("─", 72))
		for i, e := range lb.Entries {
			bm := fmt.Sprintf("%dms", e.BenchmarkMs)
			if e.BenchmarkMs == 0 {
				bm = "-"
			}
			rating := e.Rating
			if rating == 0 {
				rating = 1200
			}
			fmt.Printf("%-4d %-20s %-8d %-8d %s\n", i+1, e.Username, e.Score, rating, bm)
		}
		fmt.Println()
		return nil
	},
}

var submissionsCmd = &cobra.Command{
	Use:   "submissions [challenge]",
	Short: "Show submission history per challenge",
	Long: `Show your submission history grouped by challenge.
Optionally filter by challenge name to see history for a specific challenge only.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()

		isAuth, err := config.IsAuthenticated()
		if err != nil {
			return fmt.Errorf("authentication check failed: %w", err)
		}
		if !isAuth {
			return fmt.Errorf("not authenticated: please run 'ft_hackthon login' first")
		}

		challengeFilter := ""
		if len(args) > 0 {
			challengeFilter = args[0]
		}

		jobs, err := apiClient.ListJobs()
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		type subEntry struct {
			JobID       string
			CommitSHA   string
			CreatedAt   string
			Passed      bool
			Points      int
			MaxPoints   int
			TestsRun    int
			TestsPassed int
		}
		challengeMap := make(map[string][]subEntry)
		for _, j := range jobs.Jobs {
			if j.Result == nil || len(j.Result.Challenges) == 0 {
				continue
			}
			for _, ch := range j.Result.Challenges {
				if challengeFilter != "" && !strings.EqualFold(ch.Name, challengeFilter) {
					continue
				}
				pts := 0
				if ch.Passed {
					pts = ch.Points
				}
				entry := subEntry{
					JobID:       j.JobID,
					CommitSHA:   truncateSHA(j.CommitSHA),
					CreatedAt:   j.CreatedAt,
					Passed:      ch.Passed,
					MaxPoints:   ch.Points,
					Points:      pts,
					TestsRun:    ch.TestsRun,
					TestsPassed: ch.TestsPassed,
				}
				challengeMap[ch.Title] = append(challengeMap[ch.Title], entry)
			}
		}

		if len(challengeMap) == 0 {
			if challengeFilter != "" {
				fmt.Printf("No submissions found for challenge %q.\n", challengeFilter)
			} else {
				fmt.Println("No submissions yet.")
			}
			return nil
		}

		fmt.Println()
		fmt.Printf("Submissions")
		if challengeFilter != "" {
			fmt.Printf(" for %q", challengeFilter)
		}
		fmt.Println()
		fmt.Println(strings.Repeat("═", 72))

		chNames := make([]string, 0, len(challengeMap))
		for name := range challengeMap {
			chNames = append(chNames, name)
		}
		sort.Strings(chNames)

		for _, title := range chNames {
			entries := challengeMap[title]
			bestPts := 0
			for _, e := range entries {
				if e.Points > bestPts {
					bestPts = e.Points
				}
			}
			fmt.Printf("\n%s (%d pts)\n", title, entries[0].MaxPoints)
			for i, e := range entries {
				status := "✓"
				if !e.Passed {
					status = "✗"
				}
				ts := e.CreatedAt
				if len(ts) > 16 {
					ts = ts[:16]
				}
				fmt.Printf("  #%d  %s  %s %3d/%d pts  %2d/%d tests  [%s]\n",
					i+1, ts, status, e.Points, e.MaxPoints, e.TestsPassed, e.TestsRun, e.CommitSHA)
			}
			bestStr := fmt.Sprintf("%d/%d pts", bestPts, entries[0].MaxPoints)
			if bestPts == entries[0].MaxPoints {
				fmt.Printf("  Best: %s ✓\n", bestStr)
			} else {
				fmt.Printf("  Best: %s ✗\n", bestStr)
			}
		}
		fmt.Println()
		return nil
	},
}

var plagiarismCmd = &cobra.Command{
	Use:   "plagiarism [hackathon]",
	Short: "Check for duplicate submissions in a hackathon",
	Long:  `Group submissions by code checksum to identify potential plagiarism.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()

		hackathon := ""
		if len(args) > 0 {
			hackathon = args[0]
		} else {
			hackathon = readSuiteConfig(".")
		}
		if hackathon == "" {
			return fmt.Errorf("no hackathon specified: provide a name or run from a directory with ft_hackthon.yml")
		}

		groups, err := apiClient.CheckPlagiarism(hackathon)
		if err != nil {
			return fmt.Errorf("failed to check plagiarism: %w", err)
		}

		if len(groups.Groups) == 0 {
			fmt.Printf("No duplicate submissions found for %s\n", hackathon)
			return nil
		}

		fmt.Println()
		fmt.Printf("🔍 Potential Plagiarism - %s\n", hackathon)
		fmt.Println(strings.Repeat("─", 72))
		for i, g := range groups.Groups {
			fmt.Printf("\nGroup %d (checksum: %s):\n", i+1, g.Checksum[:12])
			for j, user := range g.Users {
				fmt.Printf("  %d. %s\n", j+1, user)
			}
		}
		fmt.Println()
		return nil
	},
}

var diffCmd = &cobra.Command{
	Use:   "diff <job_id>",
	Short: "View code submitted for a grading job",
	Long: `Show the code that was submitted as part of a grading job,
using the commit SHA stored in the job record.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()

		isAuth, err := config.IsAuthenticated()
		if err != nil {
			return fmt.Errorf("authentication check failed: %w", err)
		}
		if !isAuth {
			return fmt.Errorf("not authenticated: please run 'ft_hackthon login' first")
		}

		jobID := args[0]
		job, err := apiClient.GetJob(jobID)
		if err != nil {
			return fmt.Errorf("failed to get job: %w", err)
		}
		if job.CommitSHA == "" {
			return fmt.Errorf("no commit SHA available for this job")
		}

		ws, err := client.WorkspaceDir()
		if err != nil {
			return fmt.Errorf("failed to get workspace: %w", err)
		}
		if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
			return fmt.Errorf("workspace not found at %s - run 'login' first", ws)
		}

		fmt.Printf("Showing code for commit %s\n\n", job.CommitSHA[:12])
		showCmd := exec.Command("git", "show", job.CommitSHA, "--no-patch", "--format=medium")
		showCmd.Dir = ws
		showCmd.Stdout = os.Stdout
		showCmd.Stderr = os.Stderr
		if err := showCmd.Run(); err != nil {
			return fmt.Errorf("failed to show commit: %w", err)
		}
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove your local authentication token",
	Long:  `Remove your stored authentication token from ~/.ft_hackthon/config.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()
		authManager := client.NewAuthManager(apiClient)

		if err := authManager.Logout(); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display the currently authenticated user",
	Long:  `Display the username of the currently logged-in user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()

		isAuth, err := config.IsAuthenticated()
		if err != nil {
			return fmt.Errorf("authentication check failed: %w", err)
		}

		if !isAuth {
			fmt.Println("Not authenticated. Run 'ft_hackthon login' to authenticate.")
			return nil
		}

		info, err := apiClient.GetUserInfo()
		if err != nil {
			cfg, cerr := config.LoadConfig()
			if cerr != nil {
				return fmt.Errorf("failed to load config: %w", cerr)
			}
			if cfg.User == "" {
				fmt.Println("No user information available")
			} else {
				fmt.Printf("Logged in as: %s\n", cfg.User)
			}
			return nil
		}
		fmt.Printf("Logged in as: %s (Rating: %d)\n", info.Username, info.Rating)
		return nil
	},
}

var ratingCmd = &cobra.Command{
	Use:   "rating",
	Short: "Display your current Elo rating",
	Long:  `Show your current Elo rating based on submission performance.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiClient := newAPIClient()

		isAuth, err := config.IsAuthenticated()
		if err != nil {
			return fmt.Errorf("authentication check failed: %w", err)
		}

		if !isAuth {
			return fmt.Errorf("not authenticated: please run 'ft_hackthon login' first")
		}

		info, err := apiClient.GetUserInfo()
		if err != nil {
			return fmt.Errorf("failed to get rating: %w", err)
		}

		fmt.Printf("Your Elo rating: %d\n", info.Rating)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the version of ft_hackthon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("ft_hackthon version 1.0.0")
		fmt.Println("Built for the ft_hackthon Hackathon Grading System")
	},
}

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Display help information",
	Long:  `Display help information for ft_hackthon.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = rootCmd.Help()
	},
}
