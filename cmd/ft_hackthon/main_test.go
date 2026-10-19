package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old
	return string(out)
}

func TestRootCmd(t *testing.T) {
	if rootCmd.Use != "ft_hackthon" {
		t.Errorf("expected ft_hackthon, got %s", rootCmd.Use)
	}
	if rootCmd.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", rootCmd.Version)
	}
}

func TestInit(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("api-url")
	if f == nil {
		t.Fatal("expected --api-url flag")
	}
	if f.DefValue != "https://localhost:8443/api/v1" {
		t.Errorf("expected default https://localhost:8443/api/v1, got %s", f.DefValue)
	}

	f = rootCmd.PersistentFlags().Lookup("verbose")
	if f == nil {
		t.Fatal("expected --verbose flag")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %s", f.DefValue)
	}
}

func TestConfigPath(t *testing.T) {
	setupConfig(t)

	cfgPath, err := config.GetConfigPath()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	home := os.Getenv("HOME")
	if !strings.HasPrefix(cfgPath, home) {
		t.Errorf("expected config path in home dir, got %s", cfgPath)
	}
}

func TestTruncateSHA(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc1234def5678", "abc1234"},
		{"short", "short"},
		{"", ""},
	}
	for _, tc := range tests {
		got := truncateSHA(tc.input)
		if got != tc.want {
			t.Errorf("truncateSHA(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestReadSuiteConfig(t *testing.T) {
	t.Run("file exists and valid", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(dir+"/ft_hackthon.yml", []byte("suite: test-suite\n"), 0644)
		suite := readSuiteConfig(dir)
		if suite != "test-suite" {
			t.Errorf("expected test-suite, got %q", suite)
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		suite := readSuiteConfig(dir)
		if suite != "" {
			t.Errorf("expected empty, got %q", suite)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(dir+"/ft_hackthon.yml", []byte(": invalid"), 0644)
		suite := readSuiteConfig(dir)
		if suite != "" {
			t.Errorf("expected empty, got %q", suite)
		}
	})
}

func TestSaveGiteaConfig(t *testing.T) {
	setupConfig(t)
	saveGiteaConfig("http://gitea.local/repo.git", "tok123")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.GiteaCloneURL != "http://gitea.local/repo.git" {
		t.Errorf("expected clone URL, got %q", cfg.GiteaCloneURL)
	}
	if cfg.GiteaToken != "tok123" {
		t.Errorf("expected token, got %q", cfg.GiteaToken)
	}
}

func TestRunCI_ArgumentErrors(t *testing.T) {
	setupConfig(t)
	t.Cleanup(func() { client.Interactive = true })

	err := runCI(rootCmd, []string{"grademe", "--json"})
	if err == nil || !strings.Contains(err.Error(), "must come before the command") {
		t.Errorf("expected misplaced-flag error, got %v", err)
	}
	if client.Interactive {
		t.Error("expected CI mode to disable interactivity")
	}

	err = runCI(rootCmd, []string{"bogus"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Errorf("expected unknown-command error, got %v", err)
	}
}

func TestApplySavedSettings(t *testing.T) {
	setupConfig(t)
	prevURL, prevInsecure := apiBaseURL, insecure
	t.Cleanup(func() {
		apiBaseURL, insecure = prevURL, prevInsecure
		rootCmd.PersistentFlags().Lookup("api-url").Changed = false
	})
	config.SaveConfig(&config.Config{APIURL: "https://saved.example/api/v1", Insecure: true})

	apiBaseURL, insecure = "https://localhost:8443/api/v1", false
	applySavedSettings(rootCmd)
	if apiBaseURL != "https://saved.example/api/v1" || !insecure {
		t.Errorf("expected saved settings, got %s insecure=%v", apiBaseURL, insecure)
	}

	rootCmd.PersistentFlags().Set("api-url", "https://flag.example/api/v1")
	applySavedSettings(rootCmd)
	if apiBaseURL != "https://flag.example/api/v1" {
		t.Errorf("explicit --api-url must win over saved settings, got %s", apiBaseURL)
	}
}

func setupConfig(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	os.Setenv("HOME", tmpDir)
	return tmpDir
}
