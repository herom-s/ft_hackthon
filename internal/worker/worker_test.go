package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/grader"
)

func setupLibftSuite(t *testing.T) (string, func()) {
	rootDir := t.TempDir()
	suiteDir := filepath.Join(rootDir, "libft")
	os.MkdirAll(suiteDir, 0755)

	os.WriteFile(filepath.Join(suiteDir, "suite.yml"), []byte("name: libft\nlanguage: c\ndetect: [libft.h]\nbuild: 'gcc -o {binary} {suite_files} {workspace_files} -lm'\nrun: '{binary}'\n"), 0644)

	testContent := `#include <stdio.h>
#include <stdlib.h>
#include <time.h>
extern size_t ft_strlen(const char *s);
extern char *ft_strcpy(char *dst, const char *src);
extern int ft_strcmp(const char *s1, const char *s2);
extern int ft_atoi(const char *str);
extern int ft_isdigit(int c);
extern int ft_isalpha(int c);
extern int ft_toupper(int c);
extern int ft_tolower(int c);
extern char *ft_strdup(const char *s);
extern char *ft_strjoin(char const *s1, char const *s2);
static int tests_run = 0, tests_failed = 0;
#define TEST(name, expr) do { tests_run++; if (!(expr)) { printf("FAIL: %s\n", name); tests_failed++; } } while(0)
int main() {
    clock_t start = clock();
    char buf[256];
    TEST("strlen", ft_strlen("hello") == 5);
    TEST("strcpy", ft_strcmp(ft_strcpy(buf, "hi"), "hi") == 0);
    TEST("strcmp", ft_strcmp("a", "b") < 0);
    TEST("atoi", ft_atoi("42") == 42);
    TEST("isdigit", ft_isdigit('5') != 0);
    TEST("isalpha", ft_isalpha('a') != 0);
    TEST("toupper", ft_toupper('a') == 'A');
    TEST("tolower", ft_tolower('A') == 'a');
    char *d = ft_strdup("test");
    if (d) { TEST("strdup", ft_strcmp(d, "test") == 0); free(d); }
    char *j = ft_strjoin("a", "b");
    if (j) { TEST("strjoin", ft_strcmp(j, "ab") == 0); free(j); }
    clock_t end = clock();
    double elapsed = ((double)(end - start)) / CLOCKS_PER_SEC * 1000;
    printf("\n%d tests, %d passed, %d failed\n", tests_run, tests_run - tests_failed, tests_failed);
    printf("Time: %.2f ms\n", elapsed);
    return tests_failed > 0 ? 1 : 0;
}`
	os.WriteFile(filepath.Join(suiteDir, "test.c"), []byte(testContent), 0644)

	oldPath := grader.SuitesPath()
	grader.SetSuitesPath(rootDir)
	return rootDir, func() { grader.SetSuitesPath(oldPath) }
}

func createLocalGitRepo(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	dir := t.TempDir()

	for name, content := range files {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	}

	for _, cmd := range [][]string{
		{"init"},
		{"config", "user.name", "test"},
		{"config", "user.email", "test@test"},
		{"add", "-A"},
		{"commit", "-m", "initial"},
	} {
		c := exec.Command("git", cmd...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", cmd, err, out)
		}
	}

	shaCmd := exec.Command("git", "--git-dir", filepath.Join(dir, ".git"), "--work-tree", dir, "rev-parse", "HEAD")
	shaOut, err := shaCmd.CombinedOutput()
	if err != nil {
		// Fallback: read HEAD ref directly
		headContent, _ := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
		refPath := strings.TrimSpace(strings.TrimPrefix(string(headContent), "ref: "))
		if !strings.HasPrefix(string(headContent), "ref: ") {
			return dir, strings.TrimSpace(string(headContent))
		}
		refContent, _ := os.ReadFile(filepath.Join(dir, ".git", refPath))
		return dir, strings.TrimSpace(string(refContent))
	}
	return dir, strings.TrimSpace(string(shaOut))
}

func TestNewWorker(t *testing.T) {
	db := database.NewInMemoryDB()
	w := NewWorker(db)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	if w.db != db {
		t.Error("expected db to be set")
	}
	if w.pollInterval != 5*time.Second {
		t.Errorf("expected 5s poll interval, got %v", w.pollInterval)
	}
}

func TestToDatabaseResult(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := ToDatabaseResult(nil)
		if result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("empty challenges", func(t *testing.T) {
		r := &grader.Result{
			ParserSuccess: true,
			BenchmarkMs:   50,
			FinalScore:    100,
			Details:       "All good",
			CodeChecksum:  "abc123",
		}
		dbResult := ToDatabaseResult(r)
		if dbResult == nil {
			t.Fatal("expected non-nil result")
		}
		if dbResult.ParserSuccess != true {
			t.Error("expected ParserSuccess true")
		}
		if dbResult.BenchmarkMs != 50 {
			t.Errorf("expected 50, got %d", dbResult.BenchmarkMs)
		}
		if dbResult.FinalScore != 100 {
			t.Errorf("expected 100, got %d", dbResult.FinalScore)
		}
		if dbResult.Details != "All good" {
			t.Errorf("expected 'All good', got %s", dbResult.Details)
		}
		if dbResult.CodeChecksum != "abc123" {
			t.Errorf("expected abc123, got %s", dbResult.CodeChecksum)
		}
		if len(dbResult.Challenges) != 0 {
			t.Errorf("expected 0 challenges, got %d", len(dbResult.Challenges))
		}
	})

	t.Run("with challenges", func(t *testing.T) {
		r := &grader.Result{
			ParserSuccess: true,
			FinalScore:    85,
			Challenges: []grader.ChallengeResult{
				{Name: "ch1", Title: "Challenge 1", Passed: true, Points: 10, TestsRun: 5, TestsPassed: 5, BenchmarkMs: 20, Details: "OK"},
				{Name: "ch2", Title: "Challenge 2", Passed: false, Points: 0, TestsRun: 3, TestsPassed: 1, BenchmarkMs: 15, Details: "FAIL"},
			},
		}
		dbResult := ToDatabaseResult(r)
		if dbResult == nil {
			t.Fatal("expected non-nil result")
		}
		if len(dbResult.Challenges) != 2 {
			t.Fatalf("expected 2 challenges, got %d", len(dbResult.Challenges))
		}
		if dbResult.Challenges[0].Name != "ch1" || dbResult.Challenges[0].Passed != true {
			t.Error("expected ch1 to pass")
		}
		if dbResult.Challenges[1].Name != "ch2" || dbResult.Challenges[1].Passed != false {
			t.Error("expected ch2 to fail")
		}
	})
}

func TestStartAndStop(t *testing.T) {
	db := database.NewInMemoryDB()
	w := NewWorker(db)

	w.Start()
	time.Sleep(50 * time.Millisecond)

	w.Stop()
	time.Sleep(50 * time.Millisecond)
}

const libftHeader = `#ifndef LIBFT_H
# define LIBFT_H
# include <stdlib.h>
size_t ft_strlen(const char *s);
char *ft_strcpy(char *dst, const char *src);
int ft_strcmp(const char *s1, const char *s2);
int ft_atoi(const char *str);
int ft_isdigit(int c);
int ft_isalpha(int c);
int ft_toupper(int c);
int ft_tolower(int c);
char *ft_strdup(const char *s);
char *ft_strjoin(char const *s1, char const *s2);
#endif
`

const libftPass = `#include "libft.h"
size_t ft_strlen(const char *s) {
    size_t i = 0;
    while (s[i]) i++;
    return i;
}
char *ft_strcpy(char *dst, const char *src) {
    char *p = dst;
    while ((*p++ = *src++));
    return dst;
}
int ft_strcmp(const char *s1, const char *s2) {
    while (*s1 && *s1 == *s2) { s1++; s2++; }
    return (unsigned char)*s1 - (unsigned char)*s2;
}
int ft_atoi(const char *str) {
    int sign = 1, n = 0;
    while (*str == ' ') str++;
    if (*str == '-' || *str == '+') { if (*str == '-') sign = -1; str++; }
    while (*str >= '0' && *str <= '9') { n = n * 10 + (*str++ - '0'); }
    return sign * n;
}
int ft_isdigit(int c) { return (c >= '0' && c <= '9'); }
int ft_isalpha(int c) { return ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')); }
int ft_toupper(int c) { return (c >= 'a' && c <= 'z') ? c - 32 : c; }
int ft_tolower(int c) { return (c >= 'A' && c <= 'Z') ? c + 32 : c; }
char *ft_strdup(const char *s) {
    size_t len = ft_strlen(s) + 1;
    char *d = malloc(len);
    if (d) ft_strcpy(d, s);
    return d;
}
char *ft_strjoin(char const *s1, char const *s2) {
    size_t l1 = ft_strlen(s1), l2 = ft_strlen(s2);
    char *r = malloc(l1 + l2 + 1);
    if (r) { ft_strcpy(r, s1); ft_strcpy(r + l1, s2); }
    return r;
}
`

const libftFail = `#include "libft.h"
size_t ft_strlen(const char *s) { return 0; }
char *ft_strcpy(char *dst, const char *src) { return dst; }
int ft_strcmp(const char *s1, const char *s2) { return 0; }
int ft_atoi(const char *str) { return 0; }
int ft_isdigit(int c) { return 0; }
int ft_isalpha(int c) { return 0; }
int ft_toupper(int c) { return c; }
int ft_tolower(int c) { return c; }
char *ft_strdup(const char *s) { return NULL; }
char *ft_strjoin(char const *s1, char const *s2) { return NULL; }
`

func TestGradeProject_Passing(t *testing.T) {
	_, restoreSuite := setupLibftSuite(t)
	defer restoreSuite()

	repoDir, sha := createLocalGitRepo(t, map[string]string{
		"ft_hackthon.yml": "suite: libft\n",
		"libft.h":         libftHeader,
		"ft_impl.c":       libftPass,
	})

	w := &Worker{}
	job := &database.Job{
		ID:            "j1",
		CommitSHA:     sha,
		GiteaCloneURL: repoDir,
	}
	result := w.gradeProject(job)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.ParserSuccess {
		t.Error("expected parser success for passing test")
	}
	if result.BenchmarkMs < 0 {
		t.Errorf("expected non-negative benchmark, got %d", result.BenchmarkMs)
	}
	if result.FinalScore <= 0 {
		t.Errorf("expected positive score, got %d", result.FinalScore)
	}
	if result.Details == "" {
		t.Error("expected non-empty details")
	}
}

func TestGradeProject_Failing(t *testing.T) {
	_, restoreSuite := setupLibftSuite(t)
	defer restoreSuite()

	repoDir, sha := createLocalGitRepo(t, map[string]string{
		"ft_hackthon.yml": "suite: libft\n",
		"libft.h":         libftHeader,
		"ft_impl.c":       libftFail,
	})

	w := &Worker{}
	job := &database.Job{
		ID:            "j2",
		CommitSHA:     sha,
		GiteaCloneURL: repoDir,
	}
	result := w.gradeProject(job)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ParserSuccess {
		t.Error("expected parser failure for failing test")
	}
	if result.FinalScore != 0 {
		t.Errorf("expected score 0 for failing test, got %d", result.FinalScore)
	}
}

func TestGradeProject_MissingFile(t *testing.T) {
	_, restoreSuite := setupLibftSuite(t)
	defer restoreSuite()

	repoDir, sha := createLocalGitRepo(t, map[string]string{
		"ft_hackthon.yml": "suite: libft\n",
		"libft.h":         libftHeader,
	})

	w := &Worker{}
	job := &database.Job{
		ID:            "j3",
		CommitSHA:     sha,
		GiteaCloneURL: repoDir,
	}
	result := w.gradeProject(job)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ParserSuccess {
		t.Error("expected parser failure when no libft .c files")
	}
	if result.FinalScore != 0 {
		t.Errorf("expected score 0 for failing test, got %d", result.FinalScore)
	}
}

func TestGradeProject_NoCloneURL(t *testing.T) {
	_, restoreSuite := setupLibftSuite(t)
	defer restoreSuite()

	w := &Worker{}
	job := &database.Job{
		ID:        "j4",
		CommitSHA: "abc123",
	}
	result := w.gradeProject(job)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ParserSuccess {
		t.Error("expected parser failure for missing clone URL")
	}
	if result.Details != "No Gitea clone URL available for this job" {
		t.Errorf("expected no clone URL message, got %s", result.Details)
	}
}
