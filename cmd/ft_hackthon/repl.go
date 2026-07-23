package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
)

var commands = []struct {
	name    string
	aliases []string
	desc    string
	usage   string
	example string
}{
	{
		name: "login", desc: "Authenticate with the server",
		usage:   "login",
		example: "  login",
	},
	{
		name: "register", desc: "Create a new account",
		usage:   "register",
		example: "  register",
	},
	{
		name: "grademe", desc: "Submit current project for grading",
		usage:   "grademe",
		example: "  grademe",
	},
	{
		name: "status", aliases: []string{"st"},
		desc:   "List your jobs or check a specific job status",
		usage:   "status [job_id]",
		example: "  status\n  status abc123def456",
	},
	{
		name: "submissions", aliases: []string{"subs"},
		desc:   "Show submission history per challenge",
		usage:   "submissions [challenge]",
		example: "  submissions\n  submissions factorial",
	},
	{
		name: "leaderboard", aliases: []string{"lb", "rank"},
		desc:   "Show top scorers for a hackathon",
		usage:   "leaderboard <hackathon>",
		example: "  leaderboard code-marathon",
	},
	{
		name: "plagiarism", aliases: []string{"dup"},
		desc:   "Check for duplicate submissions",
		usage:   "plagiarism <hackathon>",
		example: "  plagiarism code-marathon",
	},
	{
		name: "diff", desc: "View code submitted for a grading job",
		usage:   "diff <job_id>",
		example: "  diff abc123def456",
	},
	{
		name: "whoami", aliases: []string{"me"},
		desc:   "Show current user",
		usage:   "whoami",
		example: "  whoami",
	},
	{
		name: "rating", aliases: []string{"elo"},
		desc:   "Display your current Elo rating",
		usage:   "rating",
		example: "  rating",
	},
	{
		name: "logout", desc: "Clear stored session",
		usage:   "logout",
		example: "  logout",
	},
	{
		name: "version", aliases: []string{"v"},
		desc:   "Display version info",
		usage:   "version",
		example: "  version",
	},
	{
		name: "help", aliases: []string{"h", "?"},
		desc:   "Show this help",
		usage:   "help [command]",
		example: "  help\n  help status",
	},
	{
		name: "exit", aliases: []string{"quit", "q"},
		desc:   "Exit the shell",
		usage:   "exit",
		example: "  exit",
	},
}

func runREPL() error {
	historyFile := filepath.Join(config.ConfigDir(), "history")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "> ",
		HistoryFile:       historyFile,
		HistorySearchFold: true,
		AutoComplete:      completer,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	apiClient := newAPIClient()
	authManager := client.NewAuthManager(apiClient)
	submitManager := client.NewSubmitManager(apiClient)
	ui := client.NewTerminalUI()

	fmt.Println("ft_hackthon interactive shell")
	if ok, _ := config.IsAuthenticated(); ok {
		if u, _ := currentUser(); u != "" {
			fmt.Printf("Logged in as %s. Type 'help' for commands.\n", u)
		}
	} else {
		fmt.Println("Not logged in. Type 'register' to create an account or 'login' if you already have one.")
	}
	fmt.Println()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]
		args := parts[1:]

		switch command {
		case "exit", "quit", "q":
			fmt.Println("Goodbye!")
			return nil

		case "help", "h", "?":
			printHelp(args)

		case "login":
			if ok, _ := config.IsAuthenticated(); ok {
				u, _ := currentUser()
				if u != "" {
					fmt.Printf("You are already logged in as %s.\n", u)
				} else {
					fmt.Println("You are already logged in.")
				}
				continue
			}
			fmt.Println("Welcome to ft_hackthon!")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")
			resp, err := authManager.Login()
			if err != nil {
				fmt.Printf("✗ Login failed: %v\n", err)
			} else {
				saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)
				if resp.GiteaCloneURL != "" {
					ws, err := ensureGiteaRepo(resp.GiteaCloneURL)
					if err != nil {
						fmt.Printf("[!] Failed to setup workspace: %v\n", err)
					} else {
						promptSuiteSelection(submitManager, ws)
					}
				}
			}

		case "register":
			if ok, _ := config.IsAuthenticated(); ok {
				u, _ := currentUser()
				if u != "" {
					fmt.Printf("You are already logged in as %s.\n", u)
				} else {
					fmt.Println("You are already logged in.")
				}
				continue
			}
			fmt.Println("Register New Account")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")
			resp, err := authManager.Register()
			if err != nil {
				fmt.Printf("✗ Registration failed: %v\n", err)
			} else {
				saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)
				if resp.GiteaCloneURL != "" {
					ws, err := ensureGiteaRepo(resp.GiteaCloneURL)
					if err != nil {
						fmt.Printf("[!] Failed to setup workspace: %v\n", err)
					} else {
						promptSuiteSelection(submitManager, ws)
					}
				}
			}

		case "grademe":
			if !checkAuth() {
				continue
			}
			if err := submitManager.SubmitGradeJob(); err != nil {
				fmt.Printf("✗ Grading failed: %v\n", err)
			}

		case "status", "st":
			if !checkAuth() {
				continue
			}
			handleStatus(apiClient, ui, args)

		case "submissions", "subs":
			if !checkAuth() {
				continue
			}
			handleSubmissions(apiClient, args)

		case "diff":
			if !checkAuth() {
				continue
			}
			handleDiff(apiClient, args)

		case "leaderboard", "lb", "rank":
			handleLeaderboard(apiClient, args)

		case "plagiarism", "dup":
			handlePlagiarism(apiClient, args)

		case "logout":
			apiClient := newAPIClient()
			am := client.NewAuthManager(apiClient)
			if err := am.Logout(); err != nil {
				fmt.Printf("✗ Logout failed: %v\n", err)
			}

		case "whoami", "me":
			if !checkAuth() {
				continue
			}
			info, err := apiClient.GetUserInfo()
			if err != nil {
				cfg, cerr := config.LoadConfig()
				if cerr != nil {
					fmt.Printf("✗ Failed to load config: %v\n", cerr)
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

		case "rating", "elo":
			if !checkAuth() {
				continue
			}
			info, err := apiClient.GetUserInfo()
			if err != nil {
				fmt.Printf("✗ Failed to get rating: %v\n", err)
				continue
			}
			fmt.Printf("Your Elo rating: %d\n", info.Rating)

		case "version", "v":
			fmt.Println("ft_hackthon version 1.0.0")

		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
		}
	}

	return nil
}

type cmdCompleter struct{}

func (c cmdCompleter) Do(line []rune, pos int) ([][]rune, int) {
	prefix := string(line[:pos])
	fields := strings.Fields(prefix)

	var suggestions [][]rune

	if len(fields) <= 1 {
		for _, cmd := range commands {
			for _, name := range append([]string{cmd.name}, cmd.aliases...) {
				if strings.HasPrefix(name, strings.ToLower(prefix)) {
					suggestions = append(suggestions, []rune(name[len(prefix):]))
				}
			}
		}
	}

	return suggestions, pos
}

var completer cmdCompleter

func printHelp(args []string) {
	if len(args) > 0 {
		for _, cmd := range commands {
			if strings.EqualFold(args[0], cmd.name) {
				printCmdHelp(cmd)
				return
			}
			for _, alias := range cmd.aliases {
				if strings.EqualFold(args[0], alias) {
					printCmdHelp(cmd)
					return
				}
			}
		}
		fmt.Printf("No help found for %q. Type 'help' to list commands.\n", args[0])
		return
	}

	fmt.Println("Available commands:")
	for _, cmd := range commands {
		names := cmd.name
		if len(cmd.aliases) > 0 {
			names += " (" + strings.Join(cmd.aliases, ", ") + ")"
		}
		fmt.Printf("  %-30s %s\n", names, cmd.desc)
	}
	fmt.Println()
	fmt.Println("Type 'help <command>' for details and examples.")
}

func printCmdHelp(cmd struct {
	name    string
	aliases []string
	desc    string
	usage   string
	example string
}) {
	fmt.Println()
	fmt.Printf("Command: %s\n", cmd.name)
	if len(cmd.aliases) > 0 {
		fmt.Printf("Aliases: %s\n", strings.Join(cmd.aliases, ", "))
	}
	fmt.Printf("Description: %s\n", cmd.desc)
	fmt.Printf("Usage: %s\n", cmd.usage)
	if cmd.example != "" {
		fmt.Println("Examples:")
		fmt.Println(cmd.example)
	}
	fmt.Println()
}

func checkAuth() bool {
	isAuth, err := config.IsAuthenticated()
	if err != nil {
		fmt.Printf("✗ Auth check failed: %v\n", err)
		return false
	}
	if !isAuth {
		fmt.Println("✗ Not authenticated. Type 'login' first.")
		return false
	}
	return true
}

func currentUser() (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.User, nil
}

func handleStatus(apiClient *client.APIClient, ui *client.TerminalUI, args []string) {
	if len(args) == 0 {
		jobs, err := apiClient.ListJobs()
		if err != nil {
			fmt.Printf("✗ Failed to list jobs: %v\n", err)
			return
		}
		if len(jobs.Jobs) == 0 {
			fmt.Println("No jobs assigned to you.")
			return
		}
		fmt.Println()
		fmt.Println("Your jobs:")
		for _, j := range jobs.Jobs {
			symbol := "•"
			switch j.Status {
			case "completed":
				symbol = "✓"
			case "failed", "error":
				symbol = "✗"
			}
			fmt.Printf("  %s %s  [%s]  %s\n", symbol, j.JobID, j.Status, j.Message)
		}
		fmt.Println()
		return
	}

	jobID := args[0]
	fmt.Printf("Checking status for job: %s\n", jobID)
	statusResp, err := apiClient.GetStatus(jobID)
	if err != nil {
		fmt.Printf("✗ Failed to get status: %v\n", err)
		return
	}
	ui.PrintStatusUpdate(statusResp)
	if statusResp.Result != nil {
		ui.PrintGradeResult(statusResp.Result)
	}
}

func handleSubmissions(apiClient *client.APIClient, args []string) {
	challengeFilter := ""
	if len(args) > 0 {
		challengeFilter = args[0]
	}

	jobs, err := apiClient.ListJobs()
	if err != nil {
		fmt.Printf("✗ Failed to list jobs: %v\n", err)
		return
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
		return
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
}

func handleDiff(apiClient *client.APIClient, args []string) {
	if len(args) == 0 {
		fmt.Println("✗ Usage: diff <job_id>")
		return
	}

	jobID := args[0]
	job, err := apiClient.GetJob(jobID)
	if err != nil {
		fmt.Printf("✗ Failed to get job: %v\n", err)
		return
	}
	if job.CommitSHA == "" {
		fmt.Println("No commit SHA available for this job.")
		return
	}

	ws, err := client.WorkspaceDir()
	if err != nil {
		fmt.Printf("✗ Failed to get workspace: %v\n", err)
		return
	}
	if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
		fmt.Println("✗ Workspace not found. Run 'login' first.")
		return
	}

	fmt.Printf("Showing code for commit %s\n\n", job.CommitSHA[:12])
	showCmd := exec.Command("git", "show", job.CommitSHA, "--no-patch", "--format=medium")
	showCmd.Dir = ws
	showCmd.Stdout = os.Stdout
	showCmd.Stderr = os.Stderr
	showCmd.Run()
}

func handleLeaderboard(apiClient *client.APIClient, args []string) {
	hackathon := ""
	if len(args) > 0 {
		hackathon = args[0]
	} else {
		hackathon = readSuiteConfig(".")
	}
	if hackathon == "" {
		fmt.Println("✗ No hackathon specified. Usage: leaderboard <hackathon>")
		return
	}
	lb, err := apiClient.GetLeaderboard(hackathon)
	if err != nil {
		fmt.Printf("✗ Failed to get leaderboard: %v\n", err)
		return
	}
	if len(lb.Entries) == 0 {
		fmt.Printf("No entries yet for %s\n", hackathon)
		return
	}
	fmt.Println()
	fmt.Printf("Leaderboard - %s\n", hackathon)
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
}

func handlePlagiarism(apiClient *client.APIClient, args []string) {
	hackathon := ""
	if len(args) > 0 {
		hackathon = args[0]
	} else {
		hackathon = readSuiteConfig(".")
	}
	if hackathon == "" {
		fmt.Println("✗ No hackathon specified. Usage: plagiarism <hackathon>")
		return
	}
	groups, err := apiClient.CheckPlagiarism(hackathon)
	if err != nil {
		fmt.Printf("✗ Failed to check plagiarism: %v\n", err)
		return
	}
	if len(groups.Groups) == 0 {
		fmt.Printf("No duplicate submissions found for %s\n", hackathon)
		return
	}
	fmt.Println()
	fmt.Printf("Potential Plagiarism - %s\n", hackathon)
	fmt.Println(strings.Repeat("─", 72))
	for i, g := range groups.Groups {
		fmt.Printf("\nGroup %d (checksum: %s):\n", i+1, g.Checksum[:12])
		for j, user := range g.Users {
			fmt.Printf("  %d. %s\n", j+1, user)
		}
	}
	fmt.Println()
}
