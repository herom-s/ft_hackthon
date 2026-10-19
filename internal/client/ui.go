package client

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type TerminalUI struct {
	spinnerIdx int
	spinner    []string
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
		fmt.Printf("  \u2192 %s\n", status.Message)
	}
}

const maxDetailLines = 8

var failureLine = regexp.MustCompile(`(?i)fail|✗|error|timed out|expected|assert|panic|segmentation|abort`)

// PrintGradeResult prints a one-line verdict, one row per challenge, and for
// failures only the lines that explain them (everything when Verbose is set).
func (ui *TerminalUI) PrintGradeResult(result *GradeResult) {
	passed := 0
	for _, ch := range result.Challenges {
		if ch.Passed {
			passed++
		}
	}
	total := len(result.Challenges)

	ok := result.ParserSuccess
	verdict := "tests failed"
	if ok {
		verdict = "tests passed"
	}
	if total > 0 {
		ok = passed == total
		verdict = fmt.Sprintf("%d/%d challenges passed", passed, total)
	}
	parts := []string{verdict, Bold(fmt.Sprintf("%d pts", result.FinalScore))}
	if result.BenchmarkMs > 0 {
		parts = append(parts, fmt.Sprintf("%d ms", result.BenchmarkMs))
	}

	fmt.Println()
	fmt.Printf("%s %s\n", statusMark(ok), strings.Join(parts, Dim(" · ")))

	filtered := false
	if total > 0 {
		fmt.Println()
		width := 0
		for _, ch := range result.Challenges {
			width = max(width, len(ch.Title))
		}
		for _, ch := range result.Challenges {
			earned := 0
			if ch.Passed {
				earned = ch.Points
			}
			pts := fmt.Sprintf("%d/%d pts", earned, ch.Points)
			fmt.Printf("  %s %-*s  %-12s %s\n", statusMark(ch.Passed), width, ch.Title, pts,
				Dim(fmt.Sprintf("%d/%d tests", ch.TestsPassed, ch.TestsRun)))
			if !ch.Passed || Verbose {
				filtered = printDetails(ch.Details, !ch.Passed) || filtered
			}
		}
	} else if !result.ParserSuccess || Verbose {
		filtered = printDetails(result.Details, !result.ParserSuccess)
	}

	fmt.Println()
	if filtered {
		hint := "Showing failing lines only. Type 'status <job_id> --verbose' for the full output."
		if !Interactive {
			hint = "Showing failing lines only. Run 'ft_hackthon --verbose status <job_id>' for the full output."
		}
		fmt.Println(Dim(hint))
	}
}

// printDetails indents test output under a challenge. For failures it keeps
// only the lines that explain the failure, capped at maxDetailLines, and
// reports whether anything was left out.
func printDetails(details string, failing bool) bool {
	var lines []string
	for _, l := range strings.Split(details, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, strings.TrimRight(l, " \r"))
		}
	}
	shown := lines
	if failing && !Verbose {
		var picked []string
		for _, l := range lines {
			if failureLine.MatchString(l) {
				picked = append(picked, l)
			}
		}
		if len(picked) > 0 {
			shown = picked
		}
		if len(shown) > maxDetailLines {
			shown = shown[:maxDetailLines]
		}
	}
	for _, l := range shown {
		if failing {
			fmt.Printf("      %s\n", Red(l))
		} else {
			fmt.Printf("      %s\n", Dim(l))
		}
	}
	if hidden := len(lines) - len(shown); hidden > 0 {
		fmt.Printf("      %s\n", Dim(fmt.Sprintf("… %d more lines", hidden)))
		return true
	}
	return false
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
		return "✓"
	case "failed", "error":
		return "✗"
	default:
		return "•"
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
