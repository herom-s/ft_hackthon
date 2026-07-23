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

	case "version":
		fmt.Println("ft_hackthon version 1.0.0")

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
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (for CI/CD)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "Run in non-interactive mode (for CI/CD)")
}
