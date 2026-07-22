package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
)

func runREPL() error {
	scanner := bufio.NewScanner(os.Stdin)
	apiClient := newAPIClient()
	authManager := client.NewAuthManager(apiClient)
	submitManager := client.NewSubmitManager(apiClient)
	ui := client.NewTerminalUI()

	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║     ft_hackthon Interactive Shell          ║")
	fmt.Println("║     Type 'help' for commands              ║")
	fmt.Println("╚════════════════════════════════════════════╝")

	fmt.Println()

	for {
		fmt.Print("ft_hackthon> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]
		args := parts[1:]

		switch command {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return nil

		case "help":
			fmt.Println()
			fmt.Println("Available commands:")
			fmt.Println("  login                 - Authenticate with the server")
			fmt.Println("  register              - Create a new account")
			fmt.Println("  grademe               - Submit current project for grading")
			fmt.Println("  status [job_id]       - List your jobs, or check a specific job status")
			fmt.Println("  submissions [challenge] - Show submission history per challenge")
			fmt.Println("  diff <job_id>         - View code submitted for a grading job")
			fmt.Println("  leaderboard <hackathon> - Show top scorers for a hackathon")
			fmt.Println("  plagiarism <hackathon>  - Check for duplicate submissions")
			fmt.Println("  logout                - Clear stored session")
			fmt.Println("  whoami                - Show current user")
			fmt.Println("  version               - Display version info")
			fmt.Println("  exit / quit           - Exit the shell")
			fmt.Println()

		case "login":
			fmt.Println("Welcome to ft_hackthon!")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")
			resp, err := authManager.Login()
			if err != nil {
				fmt.Printf("❌ Login failed: %v\n", err)
			} else {
				saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)
				if resp.GiteaCloneURL != "" {
					ws, err := ensureGiteaRepo(resp.GiteaCloneURL)
					if err != nil {
						fmt.Printf("⚠ Failed to setup workspace: %v\n", err)
					} else {
						promptSuiteSelection(submitManager, ws)
					}
				}
			}

		case "register":
			fmt.Println("Register New Account")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")
			resp, err := authManager.Register()
			if err != nil {
				fmt.Printf("❌ Registration failed: %v\n", err)
			} else {
				saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)
				if resp.GiteaCloneURL != "" {
					ws, err := ensureGiteaRepo(resp.GiteaCloneURL)
					if err != nil {
						fmt.Printf("⚠ Failed to setup workspace: %v\n", err)
					} else {
						promptSuiteSelection(submitManager, ws)
					}
				}
			}

		case "grademe":
			isAuth, err := config.IsAuthenticated()
			if err != nil {
				fmt.Printf("❌ Auth check failed: %v\n", err)
				continue
			}
			if !isAuth {
				fmt.Println("❌ Not authenticated. Type 'login' first.")
				continue
			}

			if err := submitManager.SubmitGradeJob(); err != nil {
				fmt.Printf("❌ Grading failed: %v\n", err)
			}

		case "status":
			isAuth, err := config.IsAuthenticated()
			if err != nil {
				fmt.Printf("❌ Auth check failed: %v\n", err)
				continue
			}
			if !isAuth {
				fmt.Println("❌ Not authenticated. Type 'login' first.")
				continue
			}

			if len(args) == 0 {
				jobs, err := apiClient.ListJobs()
				if err != nil {
					fmt.Printf("❌ Failed to list jobs: %v\n", err)
					continue
				}
				if len(jobs.Jobs) == 0 {
					fmt.Println("No jobs assigned to you.")
					continue
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
				continue
			}

			jobID := args[0]
			fmt.Printf("Checking status for job: %s\n", jobID)
			statusResp, err := apiClient.GetStatus(jobID)
			if err != nil {
				fmt.Printf("❌ Failed to get status: %v\n", err)
				continue
			}
			ui.PrintStatusUpdate(statusResp)
			if statusResp.Result != nil {
				ui.PrintGradeResult(statusResp.Result)
			}

		case "submissions":
			isAuth, err := config.IsAuthenticated()
			if err != nil {
				fmt.Printf("❌ Auth check failed: %v\n", err)
				continue
			}
			if !isAuth {
				fmt.Println("❌ Not authenticated. Type 'login' first.")
				continue
			}

			challengeFilter := ""
			if len(args) > 0 {
				challengeFilter = args[0]
			}

			jobs, err := apiClient.ListJobs()
			if err != nil {
				fmt.Printf("❌ Failed to list jobs: %v\n", err)
				continue
			}

			type subEntry struct {
				JobID       string
				CommitSHA   string
				CreatedAt   string
				Passed      bool
				Points      int
				TestsRun    int
				TestsPassed int
				BenchmarkMs int
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
					entry := subEntry{
						JobID:       j.JobID,
						CommitSHA:   truncateSHA(j.CommitSHA),
						CreatedAt:   j.CreatedAt,
						Passed:      ch.Passed,
						Points:      ch.Points,
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
				continue
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
				bestPassed := false
				for _, e := range entries {
					p := 0
					if e.Passed {
						p = e.Points
					}
					if p > bestPts {
						bestPts = p
						bestPassed = e.Passed
					}
				}
				fmt.Printf("\n%s (%d pts)\n", title, entries[0].Points)
				for i, e := range entries {
					status := "✓"
					if !e.Passed {
						status = "✗"
					}
					ts := e.CreatedAt
					if len(ts) > 16 {
						ts = ts[:16]
					}
					pts := e.Points
					if !e.Passed {
						pts = 0
					}
					fmt.Printf("  #%d  %s  %s %3d/%d pts  %2d/%d tests  [%s]\n",
						i+1, ts, status, pts, e.Points, e.TestsPassed, e.TestsRun, e.CommitSHA)
				}
				bestStr := fmt.Sprintf("%d/%d pts", bestPts, entries[0].Points)
				if bestPassed {
					fmt.Printf("  Best: %s ✓\n", bestStr)
				} else {
					fmt.Printf("  Best: %s ✗\n", bestStr)
				}
			}
			fmt.Println()

		case "diff":
			isAuth, err := config.IsAuthenticated()
			if err != nil {
				fmt.Printf("❌ Auth check failed: %v\n", err)
				continue
			}
			if !isAuth {
				fmt.Println("❌ Not authenticated. Type 'login' first.")
				continue
			}

			if len(args) == 0 {
				fmt.Println("❌ Usage: diff <job_id>")
				continue
			}

			jobID := args[0]
			job, err := apiClient.GetJob(jobID)
			if err != nil {
				fmt.Printf("❌ Failed to get job: %v\n", err)
				continue
			}
			if job.CommitSHA == "" {
				fmt.Println("No commit SHA available for this job.")
				continue
			}

			ws, err := client.WorkspaceDir()
			if err != nil {
				fmt.Printf("❌ Failed to get workspace: %v\n", err)
				continue
			}
			if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
				fmt.Println("❌ Workspace not found. Run 'login' first.")
				continue
			}

			fmt.Printf("Showing code for commit %s\n\n", job.CommitSHA[:12])
			showCmd := exec.Command("git", "show", job.CommitSHA, "--no-patch", "--format=medium")
			showCmd.Dir = ws
			showCmd.Stdout = os.Stdout
			showCmd.Stderr = os.Stderr
			showCmd.Run()

		case "leaderboard":
			hackathon := ""
			if len(args) > 0 {
				hackathon = args[0]
			} else {
				hackathon = readSuiteConfig(".")
			}
			if hackathon == "" {
				fmt.Println("❌ No hackathon specified. Usage: leaderboard <hackathon>")
				continue
			}
			lb, err := apiClient.GetLeaderboard(hackathon)
			if err != nil {
				fmt.Printf("❌ Failed to get leaderboard: %v\n", err)
				continue
			}
			if len(lb.Entries) == 0 {
				fmt.Printf("No entries yet for %s\n", hackathon)
				continue
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

		case "plagiarism":
			hackathon := ""
			if len(args) > 0 {
				hackathon = args[0]
			} else {
				hackathon = readSuiteConfig(".")
			}
			if hackathon == "" {
				fmt.Println("❌ No hackathon specified. Usage: plagiarism <hackathon>")
				continue
			}
			groups, err := apiClient.CheckPlagiarism(hackathon)
			if err != nil {
				fmt.Printf("❌ Failed to check plagiarism: %v\n", err)
				continue
			}
			if len(groups.Groups) == 0 {
				fmt.Printf("No duplicate submissions found for %s\n", hackathon)
				continue
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

		case "logout":
			apiClient := newAPIClient()
			am := client.NewAuthManager(apiClient)
			if err := am.Logout(); err != nil {
				fmt.Printf("❌ Logout failed: %v\n", err)
			}

		case "whoami":
			isAuth, err := config.IsAuthenticated()
			if err != nil {
				fmt.Printf("❌ Auth check failed: %v\n", err)
				continue
			}
			if !isAuth {
				fmt.Println("Not authenticated. Type 'login' to authenticate.")
				continue
			}
			info, err := apiClient.GetUserInfo()
			if err != nil {
				cfg, cerr := config.LoadConfig()
				if cerr != nil {
					fmt.Printf("❌ Failed to load config: %v\n", cerr)
					continue
				}
				if cfg.User == "" {
					fmt.Println("No user information available")
				} else {
					fmt.Printf("Logged in as: %s\n", cfg.User)
				}
			} else {
				fmt.Printf("Logged in as: %s\n", info.Username)
			}

		case "rating":
			isAuth, err := config.IsAuthenticated()
			if err != nil {
				fmt.Printf("❌ Auth check failed: %v\n", err)
				continue
			}
			if !isAuth {
				fmt.Println("Not authenticated. Type 'login' to authenticate.")
				continue
			}
			info, err := apiClient.GetUserInfo()
			if err != nil {
				fmt.Printf("❌ Failed to get rating: %v\n", err)
				continue
			}
			fmt.Printf("Your Elo rating: %d\n", info.Rating)

		case "version":
			fmt.Println("ft_hackthon version 1.0.0")
			fmt.Println("Built for the ft_hackthon Hackathon Grading System")

		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
		}
	}

	return scanner.Err()
}
