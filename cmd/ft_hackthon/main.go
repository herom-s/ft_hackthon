package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
	"github.com/spf13/cobra"
)

var (
	apiBaseURL      string
	verbose         bool
	insecure        bool
	jsonOutput      bool
	quiet           bool
	nonInteractive  bool
)

var rootCmd = &cobra.Command{
	Use:   "ft_hackthon",
	Short: "ft_hackthon Hackathon Grading System - CLI Client",
	Long: `ft_hackthon is a terminal CLI tool for submitting and grading
hackathon projects using the ft_hackthon automated grading system.

Run without arguments to start the interactive shell.`,
	Version: "1.0.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		if nonInteractive {
			return runNonInteractive(args)
		}
		return runREPL()
	},
}

func newAPIClient() *client.APIClient {
	c := client.NewAPIClient(apiBaseURL)
	if insecure {
		c.SetInsecureSkipVerify()
	}
	return c
}

func runNonInteractive(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ft_hackthon [--non-interactive] <command> [args...]")
	}

	apiClient := newAPIClient()
	submitManager := client.NewSubmitManager(apiClient)
	ui := client.NewTerminalUI()

	switch args[0] {
	case "grademe":
		if !checkAuth() {
			os.Exit(1)
		}
		err := submitManager.SubmitGradeJob()
		if jsonOutput {
			result := map[string]interface{}{
				"success": err == nil,
			}
			if err != nil {
				result["error"] = err.Error()
			}
			json.NewEncoder(os.Stdout).Encode(result)
		}
		if err != nil {
			os.Exit(1)
		}

	case "batch":
		if !checkAuth() {
			os.Exit(1)
		}
		if len(args) < 2 {
			return fmt.Errorf("usage: ft_hackthon batch <dir1> [dir2 ...]")
		}
		if args[1] == "--all-commits" {
			if len(args) < 3 {
				return fmt.Errorf("usage: ft_hackthon batch --all-commits <dir>")
			}
			results := submitManager.SubmitAllCommits(args[2])
			if jsonOutput {
				json.NewEncoder(os.Stdout).Encode(results)
				return nil
			}
			printBatchResults(results)
		} else {
			results := submitManager.BatchSubmit(args[1:], false)
			if jsonOutput {
				json.NewEncoder(os.Stdout).Encode(results)
				return nil
			}
			printBatchResults(results)
		}

	case "status":
		if !checkAuth() {
			os.Exit(1)
		}
		jobID := ""
		if len(args) > 1 {
			jobID = args[1]
		}

		if jobID == "" {
			jobs, err := apiClient.ListJobs()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if jsonOutput {
				json.NewEncoder(os.Stdout).Encode(jobs)
				return nil
			}
			handleStatus(apiClient, ui, nil)
		} else {
			statusResp, err := apiClient.GetStatus(jobID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if jsonOutput {
				json.NewEncoder(os.Stdout).Encode(statusResp)
				return nil
			}
			ui.PrintStatusUpdate(statusResp)
			if statusResp.Result != nil {
				ui.PrintGradeResult(statusResp.Result)
			}
		}

	case "submissions":
		if !checkAuth() {
			os.Exit(1)
		}
		if jsonOutput {
			jobs, err := apiClient.ListJobs()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			json.NewEncoder(os.Stdout).Encode(jobs)
			return nil
		}
		handleSubmissions(apiClient, args[1:])

	case "diff":
		if !checkAuth() {
			os.Exit(1)
		}
		if len(args) < 2 {
			return fmt.Errorf("usage: ft_hackthon diff <job_id>")
		}
		handleDiff(apiClient, args[1:])

	case "leaderboard":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: leaderboard requires a hackathon name")
			os.Exit(1)
		}
		lb, err := apiClient.GetLeaderboard(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(lb)
			return nil
		}
		handleLeaderboard(apiClient, args[1:])

	case "plagiarism":
		if len(args) < 2 {
			return fmt.Errorf("usage: ft_hackthon plagiarism <hackathon>")
		}
		if jsonOutput {
			groups, err := apiClient.CheckPlagiarism(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			json.NewEncoder(os.Stdout).Encode(groups)
			return nil
		}
		handlePlagiarism(apiClient, args[1:])

	case "report":
		if !checkAuth() {
			os.Exit(1)
		}
		opts := client.ReportOptions{DaysBack: 30}
		for i := 1; i < len(args); i++ {
			if args[i] == "--trend" {
				opts.ShowTrend = true
			} else if len(args[i]) > 7 && args[i][:7] == "--days=" {
				fmt.Sscanf(args[i], "--days=%d", &opts.DaysBack)
			} else {
				opts.ChallengeFilter = args[i]
			}
		}
		if err := submitManager.GenerateReport(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "whoami":
		if !checkAuth() {
			os.Exit(1)
		}
		info, err := apiClient.GetUserInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(info)
			return nil
		}
		fmt.Printf("Logged in as: %s (Rating: %d)\n", info.Username, info.Rating)

	case "rating":
		if !checkAuth() {
			os.Exit(1)
		}
		info, err := apiClient.GetUserInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(map[string]int{"rating": info.Rating})
			return nil
		}
		fmt.Printf("Your Elo rating: %d\n", info.Rating)

	case "logout":
		am := client.NewAuthManager(apiClient)
		if err := am.Logout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "login":
		if ok, _ := config.IsAuthenticated(); ok {
			fmt.Println("You are already logged in.")
			return nil
		}
		am := client.NewAuthManager(apiClient)
		resp, err := am.Login()
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)

	case "register":
		if ok, _ := config.IsAuthenticated(); ok {
			fmt.Println("You are already logged in.")
			return nil
		}
		am := client.NewAuthManager(apiClient)
		resp, err := am.Register()
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		saveGiteaConfig(resp.GiteaCloneURL, resp.GiteaToken)

	case "version":
		fmt.Println("ft_hackthon version 1.0.0")

	case "help":
		if len(args) > 1 {
			printHelp(args[1:])
		} else {
			printHelp(nil)
		}

	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}

	return nil
}

func main() {
	if err := config.EnsureConfigDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiBaseURL, "api-url", "https://localhost:8443/api/v1", "API base URL")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS verification (also switches https:// to http://)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (for CI/CD)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "Run in non-interactive mode (for CI/CD)")
}
