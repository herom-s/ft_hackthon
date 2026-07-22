package client

import (
	"fmt"
	"strings"
	"time"
)

// TerminalUI handles all terminal output formatting
type TerminalUI struct{}

// NewTerminalUI creates a new terminal UI instance
func NewTerminalUI() *TerminalUI {
	return &TerminalUI{}
}

// PrintStatusUpdate prints a clean status update to the terminal
func (ui *TerminalUI) PrintStatusUpdate(status *StatusResponse) {
	emoji := getStatusEmoji(status.Status)
	message := getStatusMessage(status.Status)

	fmt.Printf("%s STATUS: %s\n", emoji, message)
	if status.Message != "" {
		fmt.Printf("  → %s\n", status.Message)
	}
}

// PrintGradeResult prints the final grading result in a formatted table
func (ui *TerminalUI) PrintGradeResult(result *GradeResult) {
	width := 50

	fmt.Println(strings.Repeat("═", width))
	fmt.Println(ui.centerText("GRADING RESULTS", width))
	fmt.Println(strings.Repeat("═", width))
	fmt.Println()

	// Parser status
	parserStatus := "✓ YES"
	if !result.ParserSuccess {
		parserStatus = "✗ NO"
	}
	ui.printTableRow("Parser Success", parserStatus, width)

	// Benchmark speed
	benchmarkStr := fmt.Sprintf("%d ms", result.BenchmarkMs)
	ui.printTableRow("Benchmark Speed", benchmarkStr, width)

	// Final score
	scoreStr := fmt.Sprintf("%d points", result.FinalScore)
	ui.printTableRow("Final Score", scoreStr, width)

	fmt.Println()
	fmt.Println(strings.Repeat("═", width))

	if result.Details != "" {
		fmt.Println("\nDetails:")
		fmt.Printf("  %s\n", result.Details)
	}

	// Per-challenge breakdown
	if len(result.Challenges) > 0 {
		fmt.Println()
		ui.PrintHeader("Challenge Results")
		for _, ch := range result.Challenges {
			status := "✓"
			if !ch.Passed {
				status = "✗"
			}
			scoreStr := fmt.Sprintf("%d/%d pts", ch.Points, ch.Points)
			if !ch.Passed {
				scoreStr = fmt.Sprintf("0/%d pts", ch.Points)
			}
			fmt.Printf(" %s %-20s %-12s %d/%d tests\n", status, ch.Title, scoreStr, ch.TestsPassed, ch.TestsRun)
			if ch.Details != "" {
				fmt.Printf("   %s\n", ch.Details)
			}
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("✓ Grading completed successfully!")
}

// printTableRow prints a single row in the results table
func (ui *TerminalUI) printTableRow(label, value string, width int) {
	availableSpace := width - len(label) - len(value) - 4 // 4 for " : " and padding
	if availableSpace < 0 {
		availableSpace = 0
	}
	dots := strings.Repeat(".", availableSpace)
	fmt.Printf(" %s %s %s\n", label, dots, value)
}

// centerText centers text within a given width
func (ui *TerminalUI) centerText(text string, width int) string {
	totalPadding := width - len(text)
	leftPadding := totalPadding / 2
	rightPadding := totalPadding - leftPadding

	return strings.Repeat(" ", leftPadding) + text + strings.Repeat(" ", rightPadding)
}

// PrintLoadingSpinner prints a simple loading spinner
func (ui *TerminalUI) PrintLoadingSpinner(message string) {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < 2; i++ {
		for _, s := range spinners {
			fmt.Printf("\r%s %s", s, message)
			time.Sleep(50 * time.Millisecond)
		}
	}
	fmt.Println()
}

// getStatusEmoji returns an emoji for the current status
func getStatusEmoji(status string) string {
	switch status {
	case "queued":
		return "⏳"
	case "processing":
		return "⚙"
	case "completed":
		return "✓"
	case "failed", "error":
		return "❌"
	default:
		return "•"
	}
}

// getStatusMessage returns a human-readable message for the current status
func getStatusMessage(status string) string {
	switch status {
	case "queued":
		return "Queued - Waiting for grader availability..."
	case "processing":
		return "Processing - Running benchmarks and tests..."
	case "completed":
		return "Completed!"
	case "failed":
		return "Failed - An error occurred during grading"
	case "error":
		return "Error - Please check your submission"
	default:
		return status
	}
}

// PrintHeader prints a section header
func (ui *TerminalUI) PrintHeader(title string) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  %s\n", title)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}
