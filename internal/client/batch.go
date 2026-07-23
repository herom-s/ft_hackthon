package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type BatchResult struct {
	Dir     string
	Commit  string
	JobID   string
	Success bool
	Error   string
}

func (sm *SubmitManager) BatchSubmit(dirs []string, parallel bool) []BatchResult {
	var results []BatchResult

	for _, dir := range dirs {
		fmt.Printf("\n[%s]\n", dir)
		result := BatchResult{Dir: dir}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			result.Error = fmt.Sprintf("resolve path: %v", err)
			results = append(results, result)
			continue
		}

		gitDir := filepath.Join(absDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			result.Error = "not a git repository"
			results = append(results, result)
			continue
		}

		sha, err := GetGitCommitSHAInDir(absDir)
		if err != nil {
			result.Error = fmt.Sprintf("get commit SHA: %v", err)
			results = append(results, result)
			continue
		}
		result.Commit = sha

		ws, err := WorkspaceDir()
		if err != nil {
			result.Error = fmt.Sprintf("workspace: %v", err)
			results = append(results, result)
			continue
		}

		if err := copyToWorkspace(absDir, ws); err != nil {
			result.Error = fmt.Sprintf("copy to workspace: %v", err)
			results = append(results, result)
			continue
		}

		commitSHA, err := PushToGitea(ws)
		if err != nil {
			result.Error = fmt.Sprintf("push to gitea: %v", err)
			results = append(results, result)
			continue
		}

		suite := readSuiteConfig(ws)

		submitResp, err := sm.apiClient.Submit(commitSHA, suite)
		if err != nil {
			result.Error = fmt.Sprintf("submit: %v", err)
			results = append(results, result)
			continue
		}

		result.JobID = submitResp.JobID
		result.Success = true
		fmt.Printf("  submitted: %s (commit: %s)\n", submitResp.JobID, commitSHA[:12])
		results = append(results, result)
	}

	return results
}

func copyToWorkspace(src, dst string) error {
	cmd := exec.Command("cp", "-a", src+"/.", dst+"/")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp: %w\n%s", err, out)
	}
	return nil
}

func (sm *SubmitManager) SubmitAllCommits(projectDir string) []BatchResult {
	var results []BatchResult

	cmd := exec.Command("git", "log", "--oneline", "--reverse", "--format=%H")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return append(results, BatchResult{
			Dir:   projectDir,
			Error: fmt.Sprintf("list commits: %v", err),
		})
	}

	commits := strings.Fields(string(out))
	if len(commits) == 0 {
		return append(results, BatchResult{
			Dir:   projectDir,
			Error: "no commits found",
		})
	}

	fmt.Printf("Submitting %d commits from %s\n", len(commits), projectDir)

	for _, sha := range commits {
		cmd := exec.Command("git", "checkout", sha)
		cmd.Dir = projectDir
		if _, err := cmd.CombinedOutput(); err != nil {
			results = append(results, BatchResult{
				Dir:   projectDir,
				Commit: sha,
				Error: fmt.Sprintf("checkout %s: %v", sha, err),
			})
			continue
		}

		ws, _ := WorkspaceDir()
		if err := copyToWorkspace(projectDir, ws); err != nil {
			continue
		}

		PushToGitea(ws)
		suite := readSuiteConfig(ws)

		resp, err := sm.apiClient.Submit(sha, suite)
		if err != nil {
			results = append(results, BatchResult{
				Dir:   projectDir,
				Commit: sha,
				Error: err.Error(),
			})
			continue
		}

		results = append(results, BatchResult{
			Dir:     projectDir,
			Commit:  sha,
			JobID:   resp.JobID,
			Success: true,
		})
		fmt.Printf("  submitted commit %s -> job %s\n", sha[:12], resp.JobID)
	}

	return results
}
