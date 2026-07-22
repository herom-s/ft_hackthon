package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDir  = ".ft_hackthon"
	configFile = "config.json"
)

// Config represents the local client configuration
type Config struct {
	Token         string `json:"token"`
	User          string `json:"user"`
	RepoPath      string `json:"repo_path,omitempty"`
	GiteaCloneURL string `json:"gitea_clone_url,omitempty"`
	GiteaToken    string `json:"gitea_token,omitempty"`
}

// GetConfigPath returns the path to the config directory
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, configDir), nil
}

// GetConfigFilePath returns the path to the config file
func GetConfigFilePath() (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configPath, configFile), nil
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}
	return os.MkdirAll(configPath, 0700)
}

// LoadConfig loads the configuration from the config file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil // Return empty config if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig(cfg *Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configPath, err := GetConfigFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// IsAuthenticated checks if the user has a valid token
func IsAuthenticated() (bool, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return false, err
	}
	return cfg.Token != "", nil
}

// GetToken retrieves the stored token
func GetToken() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.Token, nil
}

// ClearToken removes the stored token
func ClearToken() error {
	cfg := &Config{}
	return SaveConfig(cfg)
}

// SaveRepoPath stores the repository path in config
func SaveRepoPath(repoPath string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.RepoPath = repoPath
	return SaveConfig(cfg)
}

// GetRepoPath retrieves the stored repository path
func GetRepoPath() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.RepoPath, nil
}
