package client

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Interactive is false when the CLI runs a single command for CI: nothing
// prompts for input and nothing redraws the terminal.
var Interactive = true

// Verbose shows full test output in grading results instead of only the
// failing lines.
var Verbose bool

var colorEnabled = detectColor()

func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return stdoutIsTerminal()
}

func stdoutIsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func SetColor(enabled bool) {
	colorEnabled = enabled
}

func paint(code, s string) string {
	if !colorEnabled {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func Green(s string) string  { return paint("32", s) }
func Red(s string) string    { return paint("31", s) }
func Yellow(s string) string { return paint("33", s) }
func Cyan(s string) string   { return paint("36", s) }
func Bold(s string) string   { return paint("1", s) }
func Dim(s string) string    { return paint("2", s) }

func statusMark(ok bool) string {
	if ok {
		return Green("✓")
	}
	return Red("✗")
}

func StatusSymbol(status string) string {
	switch status {
	case "completed":
		return Green(getStatusSymbol(status))
	case "failed", "error":
		return Red(getStatusSymbol(status))
	default:
		return Yellow(getStatusSymbol(status))
	}
}

func OKf(format string, args ...any) {
	fmt.Printf("%s %s\n", Green("✓"), fmt.Sprintf(format, args...))
}

// Warnf prints a warning line, followed by a hint when an argument is an error
// with a known fix.
func Warnf(format string, args ...any) {
	fmt.Printf("%s %s\n", Yellow("!"), fmt.Sprintf(format, args...))
	printHint(args)
}

// Failf prints an error line, followed by a hint when an argument is an error
// with a known fix.
func Failf(format string, args ...any) {
	fmt.Printf("%s %s\n", Red("✗"), fmt.Sprintf(format, args...))
	printHint(args)
}

// reportedError marks an error whose message was already shown to the user,
// so callers keep it for the exit status without printing it a second time.
type reportedError struct{ error }

func (e reportedError) Unwrap() error { return e.error }

func AlreadyReported(err error) bool {
	var r reportedError
	return errors.As(err, &r)
}

func printHint(args []any) {
	for _, a := range args {
		if err, ok := a.(error); ok {
			if h := Hint(err); h != "" {
				fmt.Println("  " + Dim(h))
			}
			return
		}
	}
}

// Hint returns a one-line suggestion for errors students commonly hit, or ""
// when there is nothing more useful to say than the error itself.
func Hint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "x509"), strings.Contains(msg, "certificate"):
		return "The server's TLS certificate is not trusted. For a local or self-signed setup, restart with --insecure."
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "client.timeout exceeded"):
		return "Can't reach the server. Check that it is running and that --api-url points to it."
	case strings.Contains(msg, "status 401"), strings.Contains(msg, "not authenticated"):
		if Interactive {
			return "Your session is missing or expired. Type 'login'."
		}
		return "Your session is missing or expired. Run 'ft_hackthon login' with FT_HACKTHON_USERNAME and FT_HACKTHON_PASSWORD set."
	case strings.Contains(msg, "workspace not found"):
		if Interactive {
			return "Type 'login' to set up your workspace."
		}
		return "Run 'ft_hackthon login' first to set up the workspace."
	}
	return ""
}
