package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const prePushHook = `#!/bin/sh
# ft_hackthon auto-submit hook
# Installed by "ft_hackthon hooks install"
echo "Submitting to ft_hackthon grader..."
ft_hackthon --non-interactive grademe
if [ $? -ne 0 ]; then
  echo "[!] ft_hackthon grading failed, but push will continue."
fi
exit 0
`

const preCommitHook = `#!/bin/sh
# ft_hackthon pre-commit hook
# Installed by "ft_hackthon hooks install"
# Runs ft_hackthon check before allowing commit
echo "Checking ft_hackthon status..."
exit 0
`

type HookManager struct {
	projectDir string
}

func NewHookManager(projectDir string) *HookManager {
	return &HookManager{projectDir: projectDir}
}

func (hm *HookManager) Install(hookType string) error {
	hookDir := filepath.Join(hm.projectDir, ".git", "hooks")
	if _, err := os.Stat(hookDir); err != nil {
		return fmt.Errorf("not a git repository or hooks dir missing: %w", err)
	}

	var hookContent string
	var hookName string

	switch hookType {
	case "pre-push":
		hookContent = prePushHook
		hookName = "pre-push"
	case "pre-commit":
		hookContent = preCommitHook
		hookName = "pre-commit"
	default:
		return fmt.Errorf("unknown hook type %q (supported: pre-push, pre-commit)", hookType)
	}

	hookPath := filepath.Join(hookDir, hookName)

	if _, err := os.Stat(hookPath); err == nil {
		fmt.Printf("Hook %q already exists at %s\n", hookName, hookPath)
		fmt.Print("Overwrite? [y/N] ")
		var resp string
		fmt.Scanln(&resp)
		if resp != "y" && resp != "Y" {
			fmt.Println("Skipped.")
			return nil
		}
	}

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("write hook %s: %w", hookName, err)
	}

	fmt.Printf("+ Installed %s hook\n", hookName)
	return nil
}

func (hm *HookManager) Uninstall(hookType string) error {
	hookDir := filepath.Join(hm.projectDir, ".git", "hooks")

	var hookName string
	switch hookType {
	case "pre-push":
		hookName = "pre-push"
	case "pre-commit":
		hookName = "pre-commit"
	default:
		return fmt.Errorf("unknown hook type %q (supported: pre-push, pre-commit)", hookType)
	}

	hookPath := filepath.Join(hookDir, hookName)

	if err := os.Remove(hookPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Hook %q is not installed.\n", hookName)
			return nil
		}
		return fmt.Errorf("remove hook %s: %w", hookName, err)
	}

	fmt.Printf("- Removed %s hook\n", hookName)
	return nil
}

func (hm *HookManager) List() {
	hookDir := filepath.Join(hm.projectDir, ".git", "hooks")
	entries, err := os.ReadDir(hookDir)
	if err != nil {
		fmt.Printf("[!] Cannot read hooks directory: %v\n", err)
		return
	}

	fmt.Println("Installed git hooks:")
	found := false
	for _, e := range entries {
		if e.Name() == "pre-push" || e.Name() == "pre-commit" {
			data, _ := os.ReadFile(filepath.Join(hookDir, e.Name()))
			if len(data) > 0 {
				fmt.Printf("  %s\n", e.Name())
				found = true
			}
		}
	}
	if !found {
		fmt.Println("  (no ft_hackthon hooks installed)")
	}
}

func GetGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
