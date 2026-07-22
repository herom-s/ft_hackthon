package grader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name          string
		parserSuccess bool
		benchmarkMs   int
		expected      int
	}{
		{"parser failed", false, 100, 0},
		{"excellent", true, 50, 100},
		{"very good", true, 100, 100},
		{"good", true, 151, 80},
		{"acceptable", true, 201, 70},
		{"needs optimization", true, 500, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateScore(tt.parserSuccess, tt.benchmarkMs)
			if score != tt.expected {
				t.Errorf("CalculateScore(%v, %d) = %d, want %d", tt.parserSuccess, tt.benchmarkMs, score, tt.expected)
			}
		})
	}
}

func TestRatingFromBenchmark(t *testing.T) {
	tests := []struct {
		ms       int
		expected string
	}{
		{50, "Excellent"},
		{100, "Excellent"},
		{110, "Very Good"},
		{150, "Very Good"},
		{180, "Good"},
		{250, "Acceptable"},
		{400, "Needs Optimization"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			r := RatingFromBenchmark(tt.ms)
			if r != tt.expected {
				t.Errorf("RatingFromBenchmark(%d) = %s, want %s", tt.ms, r, tt.expected)
			}
		})
	}
}

func TestLoadWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid config", func(t *testing.T) {
		os.WriteFile(filepath.Join(dir, "ft_hackthon.yml"), []byte("suite: libft-tester\n"), 0644)
		cfg, err := LoadWorkspaceConfig(dir)
		if err != nil {
			t.Fatalf("LoadWorkspaceConfig failed: %v", err)
		}
		if cfg.Suite != "libft-tester" {
			t.Errorf("expected 'libft-tester', got %s", cfg.Suite)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadWorkspaceConfig(t.TempDir())
		if err == nil {
			t.Error("expected error for missing ft_hackthon.yml")
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		os.WriteFile(filepath.Join(dir, "ft_hackthon.yml"), []byte("suite: [unclosed\n"), 0644)
		_, err := LoadWorkspaceConfig(dir)
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})
}

func TestSaveWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()
	err := SaveWorkspaceConfig(dir, "my-suite")
	if err != nil {
		t.Fatalf("SaveWorkspaceConfig failed: %v", err)
	}

	cfg, err := LoadWorkspaceConfig(dir)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig failed: %v", err)
	}
	if cfg.Suite != "my-suite" {
		t.Errorf("expected 'my-suite', got %s", cfg.Suite)
	}
}

func TestLoadSuiteByName(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "mysuite"), 0755)
	os.WriteFile(filepath.Join(root, "mysuite", "suite.yml"), []byte("name: mysuite\ndetect: [a.h]\nbuild: gcc\nrun: ./out\n"), 0644)

	SetSuitesPath(root)
	defer SetSuitesPath("")

	s := LoadSuiteByName("mysuite")
	if s == nil {
		t.Fatal("expected suite")
	}
	if s.Name != "mysuite" {
		t.Errorf("expected 'mysuite', got %s", s.Name)
	}

	s = LoadSuiteByName("nonexistent")
	if s != nil {
		t.Error("expected nil for nonexistent suite")
	}

	SetSuitesPath("")
	s = LoadSuiteByName("mysuite")
	if s != nil {
		t.Error("expected nil when suites path is empty")
	}
}

func TestListSuites(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "suite-a"), 0755)
	os.WriteFile(filepath.Join(root, "suite-a", "suite.yml"), []byte("name: suite-a\ndetect: []\nbuild: ''\nrun: 'true'\n"), 0644)
	os.MkdirAll(filepath.Join(root, "suite-b"), 0755)
	os.WriteFile(filepath.Join(root, "suite-b", "suite.yml"), []byte("name: suite-b\ndetect: []\nbuild: ''\nrun: 'true'\n"), 0644)
	os.MkdirAll(filepath.Join(root, "not-a-suite"), 0755)

	SetSuitesPath(root)
	defer SetSuitesPath("")

	names := ListSuites()
	if len(names) != 2 {
		t.Fatalf("expected 2 suites, got %v", names)
	}
}

func TestGrade_WithExplicitConfig(t *testing.T) {
	suiteRoot := t.TempDir()
	os.MkdirAll(filepath.Join(suiteRoot, "my-tester"), 0755)
	os.WriteFile(filepath.Join(suiteRoot, "my-tester", "suite.yml"), []byte("name: my-tester\nbuild: 'gcc -o {binary} {suite_files} {workspace_files} -lm'\nrun: '{binary}'\n"), 0644)
	os.WriteFile(filepath.Join(suiteRoot, "my-tester", "test.c"), []byte(`
#include <stdio.h>
extern int add(int a, int b);
int main() {
    if (add(2,3) == 5) { printf("1 tests, 1 passed, 0 failed\n"); return 0; }
    printf("1 tests, 0 passed, 1 failed\n"); return 1;
}`), 0644)

	SetSuitesPath(suiteRoot)
	defer SetSuitesPath("")

	ws := t.TempDir()
	SaveWorkspaceConfig(ws, "my-tester")
	os.WriteFile(filepath.Join(ws, "solution.c"), []byte("int add(int a, int b) { return a + b; }"), 0644)

	result := Grade(ws, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.ParserSuccess {
		t.Errorf("expected passing grade with explicit suite config, got: %s", result.Details)
	}
}

func TestGrade_WorkspaceConfigFallsBack(t *testing.T) {
	suiteRoot := createSuiteRoot(t, "fallback", "needed.h",
		"name: fallback\nlanguage: c\ndetect: [needed.h]\nbuild: 'gcc -o {binary} {suite_files} {workspace_files} -lm'\nrun: '{binary}'\n",
		`int main(){return 0;}`)

	SetSuitesPath(suiteRoot)
	defer SetSuitesPath("")

	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "ft_hackthon.yml"), []byte("# empty config\n"), 0644)
	os.WriteFile(filepath.Join(ws, "needed.h"), []byte(""), 0644)

	result := Grade(ws, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.ParserSuccess {
		t.Error("expected passing grade via fallback detection")
	}
}

func TestDetectByFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.c"), []byte("int main(){}"), 0644)
	os.WriteFile(filepath.Join(dir, "test.py"), []byte("print('hi')"), 0644)

	if !DetectByFile(dir, "test.c") {
		t.Error("expected DetectByFile to find test.c")
	}
	if !DetectByFile(dir, "test.py") {
		t.Error("expected DetectByFile to find test.py")
	}
	if DetectByFile(dir, "nonexistent.go") {
		t.Error("expected DetectByFile to not find nonexistent.go")
	}
}

func TestDetectSuite(t *testing.T) {
	suiteRoot := createSuiteRoot(t, "libft", "libft.h",
		"name: libft\nlanguage: c\ndetect: [libft.h]\nbuild: gcc\nrun: run\n",
		`int main(){return 0;}`)

	SetSuitesPath(suiteRoot)
	defer SetSuitesPath("")

	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "libft.h"), []byte(""), 0644)

	suite := DetectSuite(ws)
	if suite == nil {
		t.Fatal("expected detected suite")
	}
	if suite.Name != "libft" {
		t.Errorf("expected 'libft', got %s", suite.Name)
	}
}

func TestDetectSuite_NoMatch(t *testing.T) {
	suiteRoot := createSuiteRoot(t, "libft", "libft.h",
		"name: libft\nlanguage: c\ndetect: [libft.h]\nbuild: gcc\nrun: run\n",
		`int main(){return 0;}`)

	SetSuitesPath(suiteRoot)
	defer SetSuitesPath("")

	ws := t.TempDir()
	suite := DetectSuite(ws)
	if suite != nil {
		t.Errorf("expected nil for non-matching workspace, got %s", suite.Name)
	}
}

func TestDetectSuite_PrefersMostSpecific(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, "simple"), 0755)
	os.WriteFile(filepath.Join(root, "simple", "suite.yml"), []byte("name: simple\ndetect: [common.h]\nbuild: ''\nrun: 'true'\n"), 0644)

	os.MkdirAll(filepath.Join(root, "specific"), 0755)
	os.WriteFile(filepath.Join(root, "specific", "suite.yml"), []byte("name: specific\ndetect: [common.h, extra.h]\nbuild: ''\nrun: 'true'\n"), 0644)

	SetSuitesPath(root)
	defer SetSuitesPath("")

	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "common.h"), []byte(""), 0644)
	os.WriteFile(filepath.Join(ws, "extra.h"), []byte(""), 0644)

	suite := DetectSuite(ws)
	if suite == nil {
		t.Fatal("expected detected suite")
	}
	if suite.Name != "specific" {
		t.Errorf("expected 'specific' (most specific), got %s", suite.Name)
	}
}

func TestDetectSuite_EmptyPath(t *testing.T) {
	SetSuitesPath("")
	defer SetSuitesPath("")

	suite := DetectSuite(t.TempDir())
	if suite != nil {
		t.Error("expected nil when suites path is empty")
	}
}

func TestSuite_Matches(t *testing.T) {
	suite := &Suite{Detect: []string{"foo.h", "bar.c"}}

	t.Run("all files present", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "foo.h"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "bar.c"), []byte(""), 0644)
		if !suite.Matches(dir) {
			t.Error("expected match when all detect files present")
		}
	})

	t.Run("missing one file", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "foo.h"), []byte(""), 0644)
		if suite.Matches(dir) {
			t.Error("expected no match when bar.c missing")
		}
	})

	t.Run("empty detect", func(t *testing.T) {
		empty := &Suite{Detect: []string{}}
		if empty.Matches(t.TempDir()) {
			t.Error("expected no match for empty detect list")
		}
	})
}

func createSuiteRoot(t *testing.T, name, detectFile, ymlTmpl, testSrc string) string {
	root := t.TempDir()
	suiteDir := filepath.Join(root, name)
	os.MkdirAll(suiteDir, 0755)
	os.WriteFile(filepath.Join(suiteDir, "suite.yml"), []byte(ymlTmpl), 0644)
	os.WriteFile(filepath.Join(suiteDir, "test.c"), []byte(testSrc), 0644)
	return root
}

func TestLoadSuite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "suite.yml"), []byte("name: test\nlanguage: c\ndetect: [foo.h]\nbuild: gcc\nrun: ./out\n"), 0644)

	s, err := LoadSuite(dir)
	if err != nil {
		t.Fatalf("LoadSuite failed: %v", err)
	}
	if s.Name != "test" {
		t.Errorf("expected name 'test', got %s", s.Name)
	}
	if s.Language != "c" {
		t.Errorf("expected language 'c', got %s", s.Language)
	}
	if len(s.Detect) != 1 || s.Detect[0] != "foo.h" {
		t.Errorf("unexpected detect files: %v", s.Detect)
	}
	if s.Build != "gcc" {
		t.Errorf("expected build 'gcc', got %s", s.Build)
	}
	if s.Run != "./out" {
		t.Errorf("expected run './out', got %s", s.Run)
	}
	if s.Dir != dir {
		t.Errorf("expected Dir %s, got %s", dir, s.Dir)
	}
}

func TestLoadSuite_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "suite.yml"), []byte("name: [unclosed\n"), 0644)

	_, err := LoadSuite(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadSuite_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadSuite(dir)
	if err == nil {
		t.Error("expected error for missing suite.yml")
	}
}

func TestGrade_Passing(t *testing.T) {
	root := t.TempDir()
	suiteDir := filepath.Join(root, "test")
	os.MkdirAll(suiteDir, 0755)
	os.WriteFile(filepath.Join(suiteDir, "suite.yml"), []byte("name: test\nbuild: 'gcc -o {binary} {suite_files} {workspace_files} -lm'\nrun: '{binary}'\n"), 0644)
	os.WriteFile(filepath.Join(suiteDir, "test.c"), []byte(`#include <stdio.h>
extern int add(int a, int b);
int main() {
    if (add(2,3) == 5) { printf("1 tests, 1 passed, 0 failed\n"); return 0; }
    printf("1 tests, 0 passed, 1 failed\n"); return 1;
}`), 0644)
	SetSuitesPath(root)
	defer SetSuitesPath("")

	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "solution.h"), []byte("int add(int a, int b);"), 0644)
	os.WriteFile(filepath.Join(ws, "solution.c"), []byte("int add(int a, int b) { return a + b; }"), 0644)
	os.WriteFile(filepath.Join(ws, "ft_hackthon.yml"), []byte("suite: test\n"), 0644)

	result := Grade(ws, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.ParserSuccess {
		t.Error("expected passing grade")
	}
	if result.FinalScore <= 0 {
		t.Errorf("expected positive score, got %d", result.FinalScore)
	}
}

func TestGrade_Failing(t *testing.T) {
	root := t.TempDir()
	suiteDir := filepath.Join(root, "test")
	os.MkdirAll(suiteDir, 0755)
	os.WriteFile(filepath.Join(suiteDir, "suite.yml"), []byte("name: test\nbuild: 'gcc -o {binary} {suite_files} {workspace_files} -lm'\nrun: '{binary}'\n"), 0644)
	os.WriteFile(filepath.Join(suiteDir, "test.c"), []byte(`#include <stdio.h>
extern int add(int a, int b);
int main() {
    if (add(2,3) == 5) { printf("1 tests, 1 passed, 0 failed\n"); return 0; }
    printf("1 tests, 0 passed, 1 failed\n"); return 1;
}`), 0644)
	SetSuitesPath(root)
	defer SetSuitesPath("")

	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "solution.h"), []byte("int add(int a, int b);"), 0644)
	os.WriteFile(filepath.Join(ws, "solution.c"), []byte("int add(int a, int b) { return 42; }"), 0644)
	os.WriteFile(filepath.Join(ws, "ft_hackthon.yml"), []byte("suite: test\n"), 0644)

	result := Grade(ws, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ParserSuccess {
		t.Error("expected failing grade")
	}
	if result.FinalScore != 0 {
		t.Errorf("expected score 0, got %d", result.FinalScore)
	}
}

func TestGrade_NoSuite(t *testing.T) {
	SetSuitesPath(t.TempDir())
	defer SetSuitesPath("")

	result := Grade(t.TempDir(), "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ParserSuccess {
		t.Error("expected failure when no suite configured")
	}
}

func TestCollectCFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.c"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "b.c"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0644)

	files := CollectCFiles(dir)
	if files == "" {
		t.Fatal("expected non-empty file list")
	}
	if !contains(files, "a.c") || !contains(files, "b.c") {
		t.Error("expected a.c and b.c in file list")
	}
}

func TestCollectSuiteFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.c"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "helper.c"), []byte(""), 0644)

	files := CollectSuiteFiles(dir)
	if files == "" {
		t.Fatal("expected non-empty file list")
	}
	if !contains(files, "test.c") || !contains(files, "helper.c") {
		t.Error("expected test.c and helper.c in file list")
	}
}

func TestSuitesPath(t *testing.T) {
	SetSuitesPath("/custom/path")
	if SuitesPath() != "/custom/path" {
		t.Errorf("expected /custom/path, got %s", SuitesPath())
	}
}

func TestIsActive(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

	t.Run("no constraints", func(t *testing.T) {
		s := &Suite{}
		active, msg := s.IsActive(now)
		if !active {
			t.Errorf("expected active, got false with msg: %s", msg)
		}
	})

	t.Run("within window", func(t *testing.T) {
		s := &Suite{
			StartsAt: "2026-07-08T00:00:00Z",
			EndsAt:   "2026-07-10T00:00:00Z",
		}
		active, msg := s.IsActive(now)
		if !active {
			t.Errorf("expected active within window, got: %s", msg)
		}
	})

	t.Run("before start", func(t *testing.T) {
		s := &Suite{
			StartsAt: "2026-07-10T00:00:00Z",
		}
		active, msg := s.IsActive(now)
		if active {
			t.Error("expected inactive before start")
		}
		if !strings.Contains(msg, "Starts at") {
			t.Errorf("expected 'Starts at' in message, got: %s", msg)
		}
	})

	t.Run("after end", func(t *testing.T) {
		s := &Suite{
			EndsAt: "2026-07-08T00:00:00Z",
		}
		active, msg := s.IsActive(now)
		if active {
			t.Error("expected inactive after end")
		}
		if !strings.Contains(msg, "Ended at") {
			t.Errorf("expected 'Ended at' in message, got: %s", msg)
		}
	})

	t.Run("invalid starts_at ignored", func(t *testing.T) {
		s := &Suite{StartsAt: "not-a-date"}
		active, _ := s.IsActive(now)
		if !active {
			t.Error("expected active when starts_at is invalid")
		}
	})
}

func TestDetectLanguage(t *testing.T) {
	suite := &Suite{
		Language: "c",
		Languages: map[string]LanguageConfig{
			"c": {
				Extension: ".c",
				Collect:   "*.c",
			},
			"python": {
				Extension: ".py",
				Collect:   "*.py",
			},
		},
	}

	t.Run("defaults to suite language when no languages configured", func(t *testing.T) {
		s := &Suite{Language: "go"}
		dir := t.TempDir()
		lang := DetectLanguage(dir, s)
		if lang != "go" {
			t.Errorf("expected 'go', got %s", lang)
		}
	})

	t.Run("detects python from .py files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')"), 0644)
		os.WriteFile(filepath.Join(dir, "util.py"), []byte("def foo(): pass"), 0644)
		lang := DetectLanguage(dir, suite)
		if lang != "python" {
			t.Errorf("expected 'python', got %s", lang)
		}
	})

	t.Run("detects c from .c files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.c"), []byte("int main(){}"), 0644)
		os.WriteFile(filepath.Join(dir, "util.c"), []byte("int foo(){return 1;}"), 0644)
		lang := DetectLanguage(dir, suite)
		if lang != "c" {
			t.Errorf("expected 'c', got %s", lang)
		}
	})

	t.Run("prefers most common extension", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.py"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "b.py"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "c.py"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "d.c"), []byte(""), 0644)
		lang := DetectLanguage(dir, suite)
		if lang != "python" {
			t.Errorf("expected 'python' (3 files), got %s", lang)
		}
	})

	t.Run("no matching files falls back to suite language", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644)
		lang := DetectLanguage(dir, suite)
		if lang != "c" {
			t.Errorf("expected 'c' (fallback), got %s", lang)
		}
	})
}

func TestWrapWithTimeout(t *testing.T) {
	t.Run("zero timeout returns unchanged", func(t *testing.T) {
		got := wrapWithTimeout("echo hello", 0)
		if got != "echo hello" {
			t.Errorf("expected 'echo hello', got %s", got)
		}
	})

	t.Run("negative timeout returns unchanged", func(t *testing.T) {
		got := wrapWithTimeout("echo hi", -1)
		if got != "echo hi" {
			t.Errorf("expected 'echo hi', got %s", got)
		}
	})

	t.Run("positive timeout wraps with timeout command", func(t *testing.T) {
		got := wrapWithTimeout("mycmd", 30)
		want := "timeout 30 sh -c 'mycmd'"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("handles single quotes in command", func(t *testing.T) {
		got := wrapWithTimeout("echo it's", 5)
		// The single quote in "it's" should be escaped
		if !strings.Contains(got, "sh -c") {
			t.Errorf("expected shell wrapper, got %s", got)
		}
	})
}

func TestShellEscape(t *testing.T) {
	t.Run("plain string", func(t *testing.T) {
		got := shellEscape("hello")
		want := "'hello'"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("string with single quote", func(t *testing.T) {
		got := shellEscape("it's")
		want := "'it'\\''s'"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		got := shellEscape("")
		want := "''"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})
}

func TestComputeChecksum(t *testing.T) {
	t.Run("empty directory returns empty hash", func(t *testing.T) {
		dir := t.TempDir()
		cs, err := ComputeChecksum(dir, "*.c")
		if err != nil {
			t.Fatalf("ComputeChecksum failed: %v", err)
		}
		if cs == "" {
			t.Error("expected non-empty SHA256 hash even for empty dir")
		}
	})

	t.Run("same content produces same checksum", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		os.WriteFile(filepath.Join(dir1, "a.c"), []byte("int main(){}"), 0644)
		os.WriteFile(filepath.Join(dir2, "a.c"), []byte("int main(){}"), 0644)

		cs1, _ := ComputeChecksum(dir1, "*.c")
		cs2, _ := ComputeChecksum(dir2, "*.c")
		if cs1 != cs2 {
			t.Errorf("expected identical checksums, got %s vs %s", cs1, cs2)
		}
	})

	t.Run("different content produces different checksum", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		os.WriteFile(filepath.Join(dir1, "a.c"), []byte("int main(){return 1;}"), 0644)
		os.WriteFile(filepath.Join(dir2, "a.c"), []byte("int main(){return 2;}"), 0644)

		cs1, _ := ComputeChecksum(dir1, "*.c")
		cs2, _ := ComputeChecksum(dir2, "*.c")
		if cs1 == cs2 {
			t.Error("expected different checksums for different content")
		}
	})

	t.Run("pattern filters files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.c"), []byte("code"), 0644)
		os.WriteFile(filepath.Join(dir, "b.py"), []byte("code"), 0644)

		csAll, _ := ComputeChecksum(dir, "*")
		csC, _ := ComputeChecksum(dir, "*.c")
		csPy, _ := ComputeChecksum(dir, "*.py")
		if csC == csAll || csPy == csAll {
			t.Error("expected filtered checksums to differ from unfiltered")
		}
	})

	t.Run("invalid glob pattern returns error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := ComputeChecksum(dir, "[")
		if err == nil {
			t.Error("expected error for invalid glob pattern")
		}
	})
}

func TestComputeNewRating(t *testing.T) {
	t.Run("default rating stays stable at 50%", func(t *testing.T) {
		r := ComputeNewRating(1200, 50)
		// 1200 rating, 50% score: expected ≈ 0.5, actual = 0.5 → small change
		if r < 1190 || r > 1210 {
			t.Errorf("expected near 1200, got %d", r)
		}
	})

	t.Run("good score increases rating", func(t *testing.T) {
		r := ComputeNewRating(1200, 100)
		if r <= 1200 {
			t.Errorf("expected rating increase for 100%% at 1200, got %d", r)
		}
	})

	t.Run("bad score decreases rating", func(t *testing.T) {
		r := ComputeNewRating(1200, 0)
		if r >= 1200 {
			t.Errorf("expected rating decrease for 0%% at 1200, got %d", r)
		}
	})

	t.Run("minimum rating floor", func(t *testing.T) {
		r := ComputeNewRating(100, 0)
		if r < 100 {
			t.Errorf("expected minimum 100, got %d", r)
		}
	})

	t.Run("high rated user loses rating for mediocre score", func(t *testing.T) {
		r1200 := ComputeNewRating(1200, 70)
		r1600 := ComputeNewRating(1600, 70)
		// 1200-rated gains rating (70% > 50% expected)
		// 1600-rated loses rating (70% < 90.9% expected)
		if r1200 <= 1200 {
			t.Errorf("expected 1200-rated to gain, got %d", r1200)
		}
		if r1600 >= 1600 {
			t.Errorf("expected 1600-rated to lose, got %d", r1600)
		}
	})
}

func TestLoadChallenge(t *testing.T) {
	t.Run("valid challenge.yml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "challenge.yml"), []byte("name: test-challenge\npoints: 10\nbuild: gcc -o {binary} {workspace_files} -lm\nrun: ./{binary}\n"), 0644)
		os.WriteFile(filepath.Join(dir, "subject.txt"), []byte("Solve this challenge"), 0644)

		ch, err := LoadChallenge(dir)
		if err != nil {
			t.Fatalf("LoadChallenge failed: %v", err)
		}
		if ch.Name != "test-challenge" {
			t.Errorf("expected 'test-challenge', got %s", ch.Name)
		}
		if ch.Points != 10 {
			t.Errorf("expected 10 points, got %d", ch.Points)
		}
		if ch.Dir != dir {
			t.Errorf("expected dir %s, got %s", dir, ch.Dir)
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "challenge.yml"), []byte("name: [unclosed\n"), 0644)
		_, err := LoadChallenge(dir)
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadChallenge(t.TempDir())
		if err == nil {
			t.Error("expected error for missing challenge.yml")
		}
	})
}

func TestLoadChallenges(t *testing.T) {
	t.Run("finds challenge subdirectories", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "challenges", "chall-a"), 0755)
		os.WriteFile(filepath.Join(root, "challenges", "chall-a", "challenge.yml"), []byte("name: chall-a\npoints: 5\n"), 0644)
		os.MkdirAll(filepath.Join(root, "challenges", "chall-b"), 0755)
		os.WriteFile(filepath.Join(root, "challenges", "chall-b", "challenge.yml"), []byte("name: chall-b\npoints: 10\n"), 0644)
		os.MkdirAll(filepath.Join(root, "challenges", "no-yml"), 0755)

		suite := &Suite{Dir: root}
		challenges := LoadChallenges(suite)
		if len(challenges) != 2 {
			t.Fatalf("expected 2 challenges, got %d", len(challenges))
		}
		names := map[string]bool{}
		for _, ch := range challenges {
			names[ch.Name] = true
		}
		if !names["chall-a"] || !names["chall-b"] {
			t.Errorf("expected chall-a and chall-b, got %v", challenges)
		}
	})

	t.Run("no challenge directories", func(t *testing.T) {
		root := t.TempDir()
		suite := &Suite{Dir: root}
		challenges := LoadChallenges(suite)
		if len(challenges) != 0 {
			t.Errorf("expected 0 challenges, got %d", len(challenges))
		}
	})
}

func TestParseTestCounts(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantRun  int
		wantPass int
	}{
		{"standard format", "5 tests, 5 passed, 0 failed", 5, 5},
		{"with prefix", "[Factorial] 5 tests, 5 passed, 0 failed", 5, 5},
		{"all failed", "3 tests, 0 passed, 3 failed", 3, 0},
		{"partial pass", "10 tests, 7 passed, 3 failed", 10, 7},
		{"extra newlines", "\n\n5 tests, 5 passed, 0 failed\n\n", 5, 5},
		{"multiline output", "Compiling...\n5 tests, 5 passed, 0 failed\nDone", 5, 5},
			{"no match", "something else entirely", 0, 0},
			{"empty output", "", 0, 0},
			{"single test pass", "1 tests, 1 passed, 0 failed", 1, 1},
			{"single test fail", "1 tests, 0 passed, 1 failed", 1, 0},
			{"M passed out of N format", "5 passed out of 10 tests", 10, 5},
		}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, passed := parseTestCounts(tt.output)
			if run != tt.wantRun {
				t.Errorf("parseTestCounts run = %d, want %d", run, tt.wantRun)
			}
			if passed != tt.wantPass {
				t.Errorf("parseTestCounts passed = %d, want %d", passed, tt.wantPass)
			}
		})
	}
}

func TestExpandPerChallenge(t *testing.T) {
	binary := "./mybinary"
	workspace := "/tmp/workspace"
	suiteDir := "/tmp/suite"
	userFiles := []string{"/tmp/workspace/main.c", "/tmp/workspace/util.c"}
	testRunner := "/tmp/suite/test.c"

	t.Run("expands {binary}", func(t *testing.T) {
		result := expandPerChallenge("{binary}", binary, workspace, suiteDir, userFiles, testRunner)
		if result != "./mybinary" {
			t.Errorf("expected ./mybinary, got %s", result)
		}
	})

	t.Run("expands {workspace}", func(t *testing.T) {
		result := expandPerChallenge("{workspace}", binary, workspace, suiteDir, userFiles, testRunner)
		if result != "/tmp/workspace" {
			t.Errorf("expected /tmp/workspace, got %s", result)
		}
	})

	t.Run("expands {workspace_files}", func(t *testing.T) {
		result := expandPerChallenge("{workspace_files}", binary, workspace, suiteDir, userFiles, testRunner)
		expected := "/tmp/workspace/main.c /tmp/workspace/util.c"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("expands {suite}", func(t *testing.T) {
		result := expandPerChallenge("{suite}", binary, workspace, suiteDir, userFiles, testRunner)
		if result != "/tmp/suite" {
			t.Errorf("expected /tmp/suite, got %s", result)
		}
	})

	t.Run("expands {suite_files}", func(t *testing.T) {
		result := expandPerChallenge("{suite_files}", binary, workspace, suiteDir, userFiles, testRunner)
		if result != "/tmp/suite/test.c" {
			t.Errorf("expected /tmp/suite/test.c, got %s", result)
		}
	})

	t.Run("no template vars", func(t *testing.T) {
		result := expandPerChallenge("echo hello", binary, workspace, suiteDir, userFiles, testRunner)
		if result != "echo hello" {
			t.Errorf("expected unchanged, got %s", result)
		}
	})
}

func TestExpandTemplate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.c"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "util.c"), []byte(""), 0644)

	result := expandTemplate("{binary} -o {workspace_files} -I{suite} {suite_files}", "/tmp/binary", dir, "/tmp/suite", "*.c")
	if !strings.Contains(result, "/tmp/binary") {
		t.Error("expected {binary} to be expanded")
	}
	if !strings.Contains(result, dir) {
		t.Error("expected {workspace} to be expanded")
	}
	if !strings.Contains(result, "/tmp/suite") {
		t.Error("expected {suite} to be expanded")
	}
}

func TestExpandDockerTemplate(t *testing.T) {
	result := expandDockerTemplate("{binary} {workspace_files}", "*.c")
	if result != "/tmp/binary /workspace/*.c" {
		t.Errorf("expected '/tmp/binary /workspace/*.c', got %q", result)
	}

	result2 := expandDockerTemplate("echo {suite} {suite_files}", "")
	if result2 != "echo /suite /suite/*.c" {
		t.Errorf("expected 'echo /suite /suite/*.c', got %q", result2)
	}

	result3 := expandDockerTemplate("no templates", "")
	if result3 != "no templates" {
		t.Errorf("expected 'no templates', got %q", result3)
	}
}

func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.c"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "b.c"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "c.c"), []byte(""), 0644)

	t.Run("collects all .c files recursively", func(t *testing.T) {
		files := CollectFiles(dir, "*.c")
		if !strings.Contains(files, "a.c") || !strings.Contains(files, "b.c") || !strings.Contains(files, "c.c") {
			t.Errorf("expected all .c files, got %s", files)
		}
	})

	t.Run("no matching files", func(t *testing.T) {
		files := CollectFiles(dir, "*.py")
		if files != "" {
			t.Errorf("expected empty, got %s", files)
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
