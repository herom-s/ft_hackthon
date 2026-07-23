package main

import (
	"fmt"
	"os"

	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
	"github.com/spf13/cobra"
)

var (
	apiBaseURL string
	verbose    bool
	insecure   bool
)

var rootCmd = &cobra.Command{
	Use:   "ft_hackthon",
	Short: "ft_hackthon Hackathon Grading System - CLI Client",
	Long: `ft_hackthon is a terminal CLI tool for submitting and grading
hackathon projects using the ft_hackthon automated grading system.

Run without arguments to start the interactive shell.`,
	Version: "1.0.0",
	RunE: func(cmd *cobra.Command, args []string) error {
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

func main() {
	if err := config.EnsureConfigDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiBaseURL, "api-url", "https://localhost:8443/api/v1", "API base URL")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")


}
