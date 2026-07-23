package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ft_hackthon/internal/client"
	"github.com/ft_hackthon/internal/config"
	"gopkg.in/yaml.v3"
)

func readSuiteConfig(dir string) string {
	cfgPath := filepath.Join(dir, "ft_hackthon.yml")
	f, err := os.Open(cfgPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	var cfg struct {
		Suite string `yaml:"suite"`
	}
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return ""
	}
	return cfg.Suite
}

func truncateSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func ensureGiteaRepo(giteaCloneURL string) (string, error) {
	ws, err := client.CloneGiteaRepo(giteaCloneURL)
	if err != nil {
		return "", fmt.Errorf("failed to clone Gitea repo: %w", err)
	}
	config.SaveRepoPath(ws)
	fmt.Printf("Cloned repository to: %s\n", ws)
	return ws, nil
}

func promptSuiteSelection(sm *client.SubmitManager, ws string) {
	if suite := readSuiteConfig(ws); suite != "" {
		return
	}
	suite, err := sm.PromptSuiteSelection()
	if err != nil {
		fmt.Printf("[!] Failed to configure test suite: %v\n", err)
		return
	}
	if suite == "" {
		return
	}
	if err := client.InitWorkspaceRepo(ws, suite); err != nil {
		fmt.Printf("[!] Failed to push initial setup: %v\n", err)
	} else {
		fmt.Println("Initial setup pushed to Gitea.")
	}
}

func saveGiteaConfig(giteaCloneURL, giteaToken string) {
	cfg, _ := config.LoadConfig()
	cfg.GiteaCloneURL = giteaCloneURL
	cfg.GiteaToken = giteaToken
	config.SaveConfig(cfg)
}
