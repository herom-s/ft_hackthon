package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	os.Setenv("HOME", tmpDir)
	return tmpDir
}

func TestGetConfigPath(t *testing.T) {
	home := setupTestDir(t)
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	expected := filepath.Join(home, configDir)
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestGetConfigFilePath(t *testing.T) {
	setupTestDir(t)
	path, err := GetConfigFilePath()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	expected := filepath.Join(os.Getenv("HOME"), configDir, configFile)
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	setupTestDir(t)
	configPath, _ := GetConfigPath()

	// Dir should not exist yet
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("expected config dir to not exist initially")
	}

	if err := EnsureConfigDir(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// Dir should now exist
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config dir to exist: %v", err)
	}

	// Calling again should not error
	if err := EnsureConfigDir(); err != nil {
		t.Errorf("expected nil on second call, got %v", err)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	setupTestDir(t)

	cfg := &Config{Token: "test-token", User: "testuser"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if loaded.Token != "test-token" {
		t.Errorf("expected test-token, got %s", loaded.Token)
	}
	if loaded.User != "testuser" {
		t.Errorf("expected testuser, got %s", loaded.User)
	}
}

func TestLoadConfig_FileNotExist(t *testing.T) {
	setupTestDir(t)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty token, got %s", cfg.Token)
	}
}

func TestIsAuthenticated(t *testing.T) {
	setupTestDir(t)

	t.Run("no config file", func(t *testing.T) {
		authed, err := IsAuthenticated()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if authed {
			t.Error("expected false when no config exists")
		}
	})

	t.Run("with token", func(t *testing.T) {
		SaveConfig(&Config{Token: "mytoken", User: "u"})
		authed, err := IsAuthenticated()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if !authed {
			t.Error("expected true when token exists")
		}
	})
}

func TestGetToken(t *testing.T) {
	setupTestDir(t)

	t.Run("no config", func(t *testing.T) {
		token, err := GetToken()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if token != "" {
			t.Errorf("expected empty, got %s", token)
		}
	})

	t.Run("with token", func(t *testing.T) {
		SaveConfig(&Config{Token: "abc"})
		token, err := GetToken()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if token != "abc" {
			t.Errorf("expected abc, got %s", token)
		}
	})
}

func TestClearToken(t *testing.T) {
	setupTestDir(t)
	SaveConfig(&Config{Token: "abc", User: "u"})

	if err := ClearToken(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty token after clear, got %s", cfg.Token)
	}
}

func TestSaveRepoPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	err := SaveRepoPath("/tmp/my/repo")
	if err != nil {
		t.Fatalf("SaveRepoPath failed: %v", err)
	}

	p, err := GetRepoPath()
	if err != nil {
		t.Fatalf("GetRepoPath failed: %v", err)
	}
	if p != "/tmp/my/repo" {
		t.Errorf("expected /tmp/my/repo, got %s", p)
	}
}

func TestLoadServerConfig_defaults(t *testing.T) {
	cfg := LoadServerConfig()
	if cfg.APIPort != "8000" {
		t.Errorf("expected 8000, got %s", cfg.APIPort)
	}
	if cfg.TestSuitesPath != "/var/ft_hackthon/testsuites" {
		t.Errorf("expected /var/ft_hackthon/testsuites, got %s", cfg.TestSuitesPath)
	}
	if cfg.GiteaURL != "http://gitea:3000" {
		t.Errorf("expected http://gitea:3000, got %s", cfg.GiteaURL)
	}
	if cfg.GiteaPublicURL != "http://localhost:3000" {
		t.Errorf("expected http://localhost:3000, got %s", cfg.GiteaPublicURL)
	}
	if cfg.GiteaOrg != "moulinerie" {
		t.Errorf("expected moulinerie, got %s", cfg.GiteaOrg)
	}
	if cfg.GiteaAdminUser != "ft_hackthon" {
		t.Errorf("expected ft_hackthon, got %s", cfg.GiteaAdminUser)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty, got %s", cfg.DatabaseURL)
	}
}

func TestLoadServerConfig_fromEnv(t *testing.T) {
	t.Setenv("API_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	t.Setenv("TESTSUITES_PATH", "/custom/tests")
	t.Setenv("GITEA_URL", "http://custom:3000")
	t.Setenv("GITEA_PUBLIC_URL", "http://custom:3000")
	t.Setenv("GITEA_ORG", "custom-org")
	t.Setenv("GITEA_ADMIN_USER", "admin")
	t.Setenv("GITEA_ADMIN_PASSWORD", "secret")

	cfg := LoadServerConfig()
	if cfg.APIPort != "9090" {
		t.Errorf("expected 9090, got %s", cfg.APIPort)
	}
	if cfg.DatabaseURL != "postgres://u:p@h:5432/db" {
		t.Errorf("expected postgres://u:p@h:5432/db, got %s", cfg.DatabaseURL)
	}
	if cfg.TestSuitesPath != "/custom/tests" {
		t.Errorf("expected /custom/tests, got %s", cfg.TestSuitesPath)
	}
	if cfg.GiteaURL != "http://custom:3000" {
		t.Errorf("expected http://custom:3000, got %s", cfg.GiteaURL)
	}
	if cfg.GiteaAdminUser != "admin" {
		t.Errorf("expected admin, got %s", cfg.GiteaAdminUser)
	}
	if cfg.GiteaAdminPassword != "secret" {
		t.Errorf("expected secret, got %s", cfg.GiteaAdminPassword)
	}
}

func TestLoadServerConfig_emptyEnvUsesDefault(t *testing.T) {
	t.Setenv("API_PORT", "")
	t.Setenv("GITEA_ADMIN_USER", "")
	cfg := LoadServerConfig()
	if cfg.APIPort != "8000" {
		t.Errorf("expected 8000 default, got %s", cfg.APIPort)
	}
	if cfg.GiteaAdminUser != "ft_hackthon" {
		t.Errorf("expected ft_hackthon default, got %s", cfg.GiteaAdminUser)
	}
}

func TestGetRepoPath_Empty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	p, err := GetRepoPath()
	if err != nil {
		t.Fatalf("GetRepoPath failed: %v", err)
	}
	if p != "" {
		t.Errorf("expected empty, got %s", p)
	}
}

func TestSaveConfig_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg1 := &Config{Token: "token1", User: "alice"}
	if err := SaveConfig(cfg1); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	cfg2 := &Config{Token: "token2", User: "bob"}
	if err := SaveConfig(cfg2); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.Token != "token2" {
		t.Errorf("expected token2, got %s", loaded.Token)
	}
	if loaded.User != "bob" {
		t.Errorf("expected bob, got %s", loaded.User)
	}
}

func TestGetToken_Empty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	token, err := GetToken()
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %s", token)
	}
}
