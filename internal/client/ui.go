package client

import (
	"fmt"
	"strings"
	"time"
)

type TerminalUI struct {
	spinnerIdx int
	spinner     []string
}

func NewTerminalUI() *TerminalUI {
	return &TerminalUI{
		spinner: []string{"-", "\\", "|", "/"},
	}
}

func (ui *TerminalUI) PrintStatusUpdate(status *StatusResponse) {
	message := getStatusMessage(status.Status)
	fmt.Printf("STATUS: %s\n", message)
	if status.Message != "" {
		fmt.Printf("  -> %s\n", status.Message)
	}
}

func (ui *TerminalUI) PrintGradeResult(result *GradeResult) {
	width := 50

	fmt.Println(strings.Repeat("=", width))
	fmt.Println(ui.centerText("GRADING RESULTS", width))
	fmt.Println(strings.Repeat("=", width))
	fmt.Println()

	parserStatus := "YES"
	if !result.ParserSuccess {
		parserStatus = "NO"
	}
	ui.printTableRow("Parser Success", parserStatus, width)

	benchmarkStr := fmt.Sprintf("%d ms", result.BenchmarkMs)
	ui.printTableRow("Benchmark Speed", benchmarkStr, width)

	scoreStr := fmt.Sprintf("%d points", result.FinalScore)
	ui.printTableRow("Final Score", scoreStr, width)

	fmt.Println()
	fmt.Println(strings.Repeat("=", width))

	if result.Details != "" {
		fmt.Println("\nDetails:")
		fmt.Printf("  %s\n", result.Details)
	}

	if len(result.Challenges) > 0 {
		fmt.Println()
		ui.PrintHeader("Challenge Results")
		for _, ch := range result.Challenges {
			status := "+"
			if !ch.Passed {
				status = "-"
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
	fmt.Println("+ Grading completed successfully!")
}

func (ui *TerminalUI) printTableRow(label, value string, width int) {
	availableSpace := width - len(label) - len(value) - 4
	if availableSpace < 0 {
		availableSpace = 0
	}
	dots := strings.Repeat(".", availableSpace)
	fmt.Printf(" %s %s %s\n", label, dots, value)
}

func (ui *TerminalUI) centerText(text string, width int) string {
	totalPadding := width - len(text)
	leftPadding := totalPadding / 2
	rightPadding := totalPadding - leftPadding
	return strings.Repeat(" ", leftPadding) + text + strings.Repeat(" ", rightPadding)
}

func (ui *TerminalUI) PrintLoadingSpinner(message string) {
	for i := 0; i < 4; i++ {
		for _, s := range ui.spinner {
			fmt.Printf("\r%s %s", s, message)
			time.Sleep(100 * time.Millisecond)
		}
	}
	fmt.Println()
}

func (ui *TerminalUI) Spin() string {
	s := ui.spinner[ui.spinnerIdx%len(ui.spinner)]
	ui.spinnerIdx++
	return s
}

func (ui *TerminalUI) PrintProgress(current, total int, prefix string) {
	pct := float64(current) / float64(total) * 100
	barWidth := 30
	filled := int(float64(barWidth) * float64(current) / float64(total))
	bar := strings.Repeat("#", filled) + strings.Repeat("-", barWidth-filled)
	fmt.Printf("\r%s [%s] %d/%d (%.0f%%)", prefix, bar, current, total, pct)
	if current >= total {
		fmt.Println()
	}
}

func getStatusSymbol(status string) string {
	switch status {
	case "completed":
		return "+"
	case "failed", "error":
		return "-"
	default:
		return "*"
	}
}

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

func (ui *TerminalUI) PrintHeader(title string) {
	fmt.Println()
	fmt.Println("--------------------------------------------")
	fmt.Printf("  %s\n", title)
	fmt.Println("--------------------------------------------")
	fmt.Println()
}
