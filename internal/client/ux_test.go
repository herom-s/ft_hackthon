package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

func withoutColor(t *testing.T) {
	t.Helper()
	prev := colorEnabled
	SetColor(false)
	t.Cleanup(func() { SetColor(prev) })
}

func TestPrintGradeResult_SummaryAndFailingLines(t *testing.T) {
	withoutColor(t)
	result := &GradeResult{
		ParserSuccess: true,
		FinalScore:    85,
		BenchmarkMs:   12,
		Challenges: []ChallengeDetail{
			{Name: "factorial", Title: "Factorial", Passed: true, Points: 85, TestsRun: 5, TestsPassed: 5, Details: "all good"},
			{Name: "fib", Title: "Fibonacci", Passed: false, Points: 15, TestsRun: 5, TestsPassed: 4,
				Details: "compiling solution\nFAIL: fib(10) expected 55, got 54\nunrelated noise\n5 tests, 4 passed, 1 failed"},
		},
	}

	out := captureOutput(t, func() { NewTerminalUI().PrintGradeResult(result) })

	for _, want := range []string{"✗ 1/2 challenges passed", "85 pts", "12 ms", "✓ Factorial", "✗ Fibonacci", "0/15 pts", "FAIL: fib(10) expected 55", "Showing failing lines only"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"unrelated noise", "all good", "Grading completed successfully"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output should not contain %q:\n%s", unwanted, out)
		}
	}

	Verbose = true
	t.Cleanup(func() { Verbose = false })
	out = captureOutput(t, func() { NewTerminalUI().PrintGradeResult(result) })
	for _, want := range []string{"unrelated noise", "all good"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Showing failing lines only") {
		t.Errorf("verbose output should not mention filtering:\n%s", out)
	}
}

func TestPrintGradeResult_AllPassed(t *testing.T) {
	withoutColor(t)
	result := &GradeResult{
		ParserSuccess: true,
		FinalScore:    100,
		Challenges:    []ChallengeDetail{{Title: "Prime", Passed: true, Points: 100, TestsRun: 3, TestsPassed: 3}},
	}
	out := captureOutput(t, func() { NewTerminalUI().PrintGradeResult(result) })
	if !strings.Contains(out, "✓ 1/1 challenges passed") {
		t.Errorf("expected passing verdict:\n%s", out)
	}
}

func statusServer(t *testing.T, ws func(*websocket.Conn), polls *atomic.Int32, authHeader *atomic.Value) (*APIClient, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws/grade/status/") {
			authHeader.Store(r.Header.Get("Authorization"))
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			ws(conn)
			return
		}
		polls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StatusResponse{JobID: "job-1", Status: "completed", Result: &GradeResult{ParserSuccess: true, FinalScore: 10}})
	})
	return c, s.Close
}

func TestPollStatus_StreamsOverWebSocket(t *testing.T) {
	setupConfig(t)
	var polls atomic.Int32
	var auth atomic.Value
	c, stop := statusServer(t, func(conn *websocket.Conn) {
		conn.WriteJSON(StatusResponse{JobID: "job-1", Status: "queued"})
		conn.WriteJSON(StatusResponse{JobID: "job-1", Status: "completed", Result: &GradeResult{ParserSuccess: true, FinalScore: 42}})
	}, &polls, &auth)
	defer stop()

	var err error
	out := captureOutput(t, func() { err = NewSubmitManager(c).PollStatus("job-1") })
	if err != nil {
		t.Fatalf("PollStatus: %v", err)
	}
	if polls.Load() != 0 {
		t.Errorf("expected no HTTP polls while the stream works, got %d", polls.Load())
	}
	if got := auth.Load(); got != "Bearer test-token" {
		t.Errorf("expected token in Authorization header, got %v", got)
	}
	if !strings.Contains(out, "42 pts") {
		t.Errorf("expected streamed result in output:\n%s", out)
	}
}

func TestPollStatus_FallsBackToPollingWhenStreamEnds(t *testing.T) {
	setupConfig(t)
	var polls atomic.Int32
	var auth atomic.Value
	c, stop := statusServer(t, func(conn *websocket.Conn) {
		conn.WriteJSON(map[string]string{"error": "Job not found"})
	}, &polls, &auth)
	defer stop()

	var err error
	captureOutput(t, func() { err = NewSubmitManager(c).PollStatus("job-1") })
	if err != nil {
		t.Fatalf("PollStatus: %v", err)
	}
	if polls.Load() == 0 {
		t.Error("expected an HTTP poll after the stream ended without a final status")
	}
}

func TestPollStatus_FailedJobIsReportedOnce(t *testing.T) {
	setupConfig(t)
	withoutColor(t)
	var polls atomic.Int32
	var auth atomic.Value
	c, stop := statusServer(t, func(conn *websocket.Conn) {
		conn.WriteJSON(StatusResponse{JobID: "job-1", Status: "failed", Message: "build exploded"})
	}, &polls, &auth)
	defer stop()

	var err error
	out := captureOutput(t, func() { err = NewSubmitManager(c).PollStatus("job-1") })
	if err == nil || !AlreadyReported(err) {
		t.Fatalf("expected an already-reported error, got %v", err)
	}
	if strings.Count(out, "build exploded") != 1 {
		t.Errorf("expected the failure message exactly once:\n%s", out)
	}
}

func TestCredentials_CIModeUsesEnvironment(t *testing.T) {
	Interactive = false
	t.Cleanup(func() { Interactive = true })

	t.Setenv("FT_HACKTHON_USERNAME", "")
	t.Setenv("FT_HACKTHON_PASSWORD", "")
	if _, _, err := credentials(); err == nil {
		t.Fatal("expected an error when CI credentials are missing")
	}

	t.Setenv("FT_HACKTHON_USERNAME", "ci-bot")
	t.Setenv("FT_HACKTHON_PASSWORD", "s3cret")
	u, p, err := credentials()
	if err != nil || u != "ci-bot" || p != "s3cret" {
		t.Fatalf("got %q %q %v", u, p, err)
	}
}

func TestHint(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.New(`Get "https://x": tls: failed to verify certificate: x509: certificate signed by unknown authority`), "--insecure"},
		{errors.New("dial tcp 127.0.0.1:8443: connect: connection refused"), "--api-url"},
		{errors.New(`Get "http://h/api/v1/user/me": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`), "--api-url"},
		{errors.New("status request failed: status 401 - unauthorized"), "login"},
		{errors.New("something else entirely"), ""},
	}
	for _, tt := range tests {
		got := Hint(tt.err)
		if tt.want == "" && got != "" {
			t.Errorf("Hint(%q) = %q, want none", tt.err, got)
		}
		if tt.want != "" && !strings.Contains(got, tt.want) {
			t.Errorf("Hint(%q) = %q, want mention of %q", tt.err, got, tt.want)
		}
	}
}

func TestAlreadyReported(t *testing.T) {
	inner := errors.New("boom")
	if !AlreadyReported(fmt.Errorf("wrapped: %w", reportedError{inner})) {
		t.Error("expected wrapped reported error to be detected")
	}
	if AlreadyReported(inner) {
		t.Error("plain error must not count as reported")
	}
}

func TestPaintRespectsColorSetting(t *testing.T) {
	prev := colorEnabled
	t.Cleanup(func() { SetColor(prev) })
	SetColor(false)
	if Red("x") != "x" {
		t.Error("expected plain text with color off")
	}
	SetColor(true)
	if Red("x") == "x" {
		t.Error("expected ANSI codes with color on")
	}
}
