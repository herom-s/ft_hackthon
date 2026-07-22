package grader

import (
	"archive/tar"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var tmpPattern = "ft-hackthon-*"

func Grade(workspaceDir string, suiteName string) *Result {
	var suite *Suite

	// Use explicit suite name if provided (takes priority)
	if suiteName != "" {
		suite = LoadSuiteByName(suiteName)
		if suite == nil {
			return &Result{
				Details: fmt.Sprintf("Suite %q from job not found in %s", suiteName, suitesPath),
			}
		}
	} else if cfg, err := LoadWorkspaceConfig(workspaceDir); err == nil && cfg.Suite != "" {
		suite = LoadSuiteByName(cfg.Suite)
		if suite == nil {
			return &Result{
				Details: fmt.Sprintf("Suite %q from %s not found in %s", cfg.Suite, workspaceConfigFile, suitesPath),
			}
		}
	} else {
		suite = DetectSuite(workspaceDir)
		if suite == nil {
			return &Result{
				Details: fmt.Sprintf("No matching test suite found in %s", suitesPath),
			}
		}
	}

	result := gradeSuite(suite, workspaceDir)
	if result != nil {
		// Compute code checksum using the suite's language pattern
		pattern := "*.c"
		lang := DetectLanguage(workspaceDir, suite)
		if lc, ok := suite.Languages[lang]; ok && lc.Collect != "" {
			pattern = lc.Collect
		}
		if cs, err := ComputeChecksum(workspaceDir, pattern); err == nil {
			result.CodeChecksum = cs
		}
	}
	return result
}

func gradeSuite(suite *Suite, workspaceDir string) *Result {
	challengesDir := filepath.Join(suite.Dir, "challenges")
	if _, err := os.Stat(challengesDir); err == nil {
		return GradeChallenges(suite, workspaceDir)
	}
	return RunSuite(suite, workspaceDir)
}

func RunSuite(suite *Suite, workspaceDir string) *Result {
	dockerfilePath := filepath.Join(suite.Dir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		return runSuiteDocker(suite, workspaceDir)
	}
	return runSuiteLocal(suite, workspaceDir)
}

func GradeChallenges(suite *Suite, workspaceDir string) *Result {
	challenges := LoadChallenges(suite)
	if len(challenges) == 0 {
		return &Result{
			Details: fmt.Sprintf("No challenges found in suite %s", suite.Name),
		}
	}

	var results []ChallengeResult
	totalPoints := 0
	earnedPoints := 0
	maxBenchMs := 0
	challengesPassed := 0

	for _, ch := range challenges {
		totalPoints += ch.Points
		chDir := filepath.Join(workspaceDir, ch.Name)
		if _, err := os.Stat(chDir); os.IsNotExist(err) {
			results = append(results, ChallengeResult{
				Name:    ch.Name,
				Title:   ch.Title,
				Passed:  false,
				Points:  ch.Points,
				Details: "not attempted",
			})
			continue
		}

		cr := gradeSingleChallenge(ch, chDir, suite)
		results = append(results, cr)
		if cr.Passed {
			earnedPoints += ch.Points
			challengesPassed++
		}
	}

	allPassed := earnedPoints == totalPoints && totalPoints > 0
	finalScore := 0
	if allPassed && totalPoints > 0 {
		finalScore = 100
	} else if totalPoints > 0 {
		finalScore = (earnedPoints * 100) / totalPoints
	}

	return &Result{
		ParserSuccess: allPassed,
		BenchmarkMs:   maxBenchMs,
		FinalScore:    finalScore,
		Details:       fmt.Sprintf("%d/%d challenges passed, %d/%d points", challengesPassed, len(challenges), earnedPoints, totalPoints),
		Challenges:    results,
	}
}

func gradeSingleChallenge(ch *Challenge, chDir string, suite *Suite) ChallengeResult {
	cr := ChallengeResult{
		Name:   ch.Name,
		Title:  ch.Title,
		Points: ch.Points,
	}

	lang := DetectLanguage(chDir, suite)
	lc, hasLangConfig := suite.Languages[lang]

	testRunnerName := "test_runner.c"
	collectPattern := "*.c"
	if hasLangConfig {
		if lc.TestRunner != "" {
			testRunnerName = lc.TestRunner
		}
		if lc.Collect != "" {
			collectPattern = lc.Collect
		}
	}

	testRunner := filepath.Join(ch.Dir, testRunnerName)
	if _, err := os.Stat(testRunner); os.IsNotExist(err) {
		cr.Details = fmt.Sprintf("No %s found for challenge", testRunnerName)
		return cr
	}

	userFiles, _ := filepath.Glob(filepath.Join(chDir, collectPattern))

	// Exclude the test runner from user files — the suite provides it
	var filtered []string
	for _, f := range userFiles {
		if filepath.Base(f) == testRunnerName {
			continue
		}
		filtered = append(filtered, f)
	}
	userFiles = filtered

	if len(userFiles) == 0 {
		cr.Details = fmt.Sprintf("No %s files found in challenge directory", collectPattern)
		return cr
	}

	buildCmdStr := suite.Build
	runCmdStr := suite.Run
	if hasLangConfig {
		if lc.Build != "" {
			buildCmdStr = lc.Build
		}
		if lc.Run != "" {
			runCmdStr = lc.Run
		}
	}

	// Use Docker if the suite has a Dockerfile
	_, dockerErr := os.Stat(filepath.Join(suite.Dir, "Dockerfile"))
	useDocker := dockerErr == nil

	if useDocker {
		return gradeSingleChallengeDocker(ch, chDir, suite, cr, buildCmdStr, runCmdStr, collectPattern, testRunner, testRunnerName, userFiles)
	}
	return gradeSingleChallengeLocal(ch, chDir, suite, cr, buildCmdStr, runCmdStr, collectPattern, testRunner, userFiles)
}

func gradeSingleChallengeLocal(ch *Challenge, chDir string, suite *Suite, cr ChallengeResult, buildCmdStr, runCmdStr, collectPattern string, testRunner string, userFiles []string) ChallengeResult {
	tmpBin, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		cr.Details = fmt.Sprintf("Failed to create temp binary: %v", err)
		return cr
	}
	tmpPath := tmpBin.Name()
	tmpBin.Close()
	defer os.Remove(tmpPath)

	if buildCmdStr != "" {
		buildCmd := expandPerChallenge(buildCmdStr, tmpPath, chDir, ch.Dir, userFiles, testRunner)
		buildCmdOut, err := exec.Command("sh", "-c", buildCmd).CombinedOutput()
		if err != nil {
			cr.Details = fmt.Sprintf("Build failed: %s", strings.TrimSpace(string(buildCmdOut)))
			return cr
		}
	}

	if runCmdStr == "" {
		cr.Details = "No run command defined"
		return cr
	}

	runCmd := expandPerChallenge(runCmdStr, tmpPath, chDir, ch.Dir, userFiles, testRunner)

	timeoutSec := suite.DefaultTimeoutSeconds
	if ch.TimeoutSeconds > 0 {
		timeoutSec = ch.TimeoutSeconds
	}
	runCmd = wrapWithTimeout(runCmd, timeoutSec)

	output, err := exec.Command("sh", "-c", runCmd).CombinedOutput()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 124 {
			cr.Details = fmt.Sprintf("Timed out after %d seconds", timeoutSec)
			return cr
		}
		cr.Details = fmt.Sprintf("Runtime error: %s", strings.TrimSpace(string(output)))
		return cr
	}

	outStr := string(output)
	cr.TestsRun, cr.TestsPassed = parseTestCounts(outStr)
	cr.Passed = cr.TestsRun > 0 && cr.TestsRun == cr.TestsPassed
	cr.Details = outStr
	return cr
}

func gradeSingleChallengeDocker(ch *Challenge, chDir string, suite *Suite, cr ChallengeResult, buildCmdStr, runCmdStr, collectPattern string, testRunner string, testRunnerName string, userFiles []string) ChallengeResult {
	imageName := "ft-hackthon-suite-" + suite.Name
	if err := buildSuiteImage(imageName, suite.Dir); err != nil {
		cr.Details = fmt.Sprintf("Failed to build suite image: %v", err)
		return cr
	}

	tmpDir, err := os.MkdirTemp("", tmpPattern)
	if err != nil {
		cr.Details = fmt.Sprintf("Failed to create temp dir: %v", err)
		return cr
	}
	defer os.RemoveAll(tmpDir)

	for _, f := range userFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			cr.Details = fmt.Sprintf("Failed to read %s: %v", f, err)
			return cr
		}
		if err := os.WriteFile(filepath.Join(tmpDir, filepath.Base(f)), data, 0644); err != nil {
			cr.Details = fmt.Sprintf("Failed to copy %s: %v", f, err)
			return cr
		}
	}

	runnerData, err := os.ReadFile(testRunner)
	if err != nil {
		cr.Details = fmt.Sprintf("Failed to read test runner: %v", err)
		return cr
	}
	if err := os.WriteFile(filepath.Join(tmpDir, testRunnerName), runnerData, 0644); err != nil {
		cr.Details = fmt.Sprintf("Failed to copy test runner: %v", err)
		return cr
	}

	// Override {suite_files} to empty — the test runner was already copied
	// into /workspace/ alongside user files, so /workspace/*.c glob catches it.
	r := strings.NewReplacer(
		"{binary}", "/tmp/binary",
		"{workspace}", "/workspace",
		"{workspace_files}", fmt.Sprintf("/workspace/%s", collectPattern),
		"{suite}", "/suite",
		"{suite_files}", "",
	)
	buildCmd := r.Replace(buildCmdStr)
	runCmd := r.Replace(runCmdStr)

	fullCmd := buildCmd
	if runCmd != "" {
		if fullCmd != "" {
			fullCmd += " && "
		}
		fullCmd += runCmd
	}
	if fullCmd == "" {
		cr.Details = "No build or run command defined"
		return cr
	}

	timeoutSec := suite.DefaultTimeoutSeconds
	if ch.TimeoutSeconds > 0 {
		timeoutSec = ch.TimeoutSeconds
	}
	memMB := suite.DefaultMemoryMB
	fullCmd = wrapWithTimeout(fullCmd, timeoutSec)

	containerName := "ft-hackthon-grade-" + suite.Name + "-" + ch.Name
	cleanupContainer(containerName)

	createArgs := []string{"create", "--name", containerName}
	if memMB > 0 {
		mem := fmt.Sprintf("%dm", memMB)
		createArgs = append(createArgs, "--memory", mem, "--memory-swap", mem)
	}
	createArgs = append(createArgs, imageName, "sh", "-c", fullCmd)

	createOut, err := exec.Command("docker", createArgs...).CombinedOutput()
	if err != nil {
		cr.Details = fmt.Sprintf("Failed to create container: %v\n%s", err, string(createOut))
		return cr
	}
	defer cleanupContainer(containerName)

	cpOut, err := exec.Command("docker", "cp",
		tmpDir+"/.", containerName+":/workspace/").CombinedOutput()
	if err != nil {
		cr.Details = fmt.Sprintf("Failed to copy workspace: %v\n%s", err, string(cpOut))
		return cr
	}

	runStart := time.Now()
	startOut, err := exec.Command("docker", "start", "-a", containerName).CombinedOutput()
	runTime := time.Since(runStart)

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 127 {
			cr.Details = fmt.Sprintf("Build/Run failed - command not found in container:\n%s", string(startOut))
			return cr
		}
	}

	outStr := string(startOut)
	cr.TestsRun, cr.TestsPassed = parseTestCounts(outStr)
	cr.Passed = cr.TestsRun > 0 && cr.TestsRun == cr.TestsPassed
	cr.BenchmarkMs = int(runTime.Milliseconds())
	cr.Details = outStr
	return cr
}

// wrapWithTimeout prefixes a shell command with timeout(1) if sec > 0.
func wrapWithTimeout(cmd string, sec int) string {
	if sec <= 0 {
		return cmd
	}
	// Use timeout from busybox/coreutils. Exit code 124 means timeout.
	return fmt.Sprintf("timeout %d sh -c %s", sec, shellEscape(cmd))
}

// shellEscape wraps a string in single quotes, handling inner quotes.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func expandPerChallenge(tmpl, binary, workspace, suiteDir string, userFiles []string, testRunner string) string {
	userFilesStr := strings.Join(userFiles, " ")
	r := strings.NewReplacer(
		"{binary}", binary,
		"{workspace}", workspace,
		"{workspace_files}", userFilesStr,
		"{suite}", suiteDir,
		"{suite_files}", testRunner,
	)
	return r.Replace(tmpl)
}

var testCountRe = regexp.MustCompile(`(\d+)\s+tests?,\s+(\d+)\s+passed?,\s+(\d+)\s+failed?`)

func parseTestCounts(output string) (total, passed int) {
	for _, line := range strings.Split(output, "\n") {
		var failed int
		_, err := fmt.Sscanf(line, "%d tests, %d passed, %d failed", &total, &passed, &failed)
		if err == nil {
			return total, passed
		}
		_, err2 := fmt.Sscanf(line, "%d passed out of %d tests", &passed, &total)
		if err2 == nil {
			return total, passed
		}
		matches := testCountRe.FindStringSubmatch(line)
		if len(matches) == 4 {
			fmt.Sscanf(matches[1], "%d", &total)
			fmt.Sscanf(matches[2], "%d", &passed)
			return total, passed
		}
	}
	return 0, 0
}

func runSuiteLocal(suite *Suite, workspaceDir string) *Result {
	lang := DetectLanguage(workspaceDir, suite)
	lc, hasLang := suite.Languages[lang]
	buildCmdStr := suite.Build
	runCmdStr := suite.Run
	collectPattern := "*.c"
	if hasLang {
		if lc.Build != "" {
			buildCmdStr = lc.Build
		}
		if lc.Run != "" {
			runCmdStr = lc.Run
		}
		if lc.Collect != "" {
			collectPattern = lc.Collect
		}
	}

	tmpBin, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		return &Result{
			Details: fmt.Sprintf("Failed to create temp binary: %v", err),
		}
	}
	tmpBin.Close()
	os.Remove(tmpBin.Name())
	binaryPath := tmpBin.Name()
	defer os.Remove(binaryPath)

	buildCmd := expandTemplate(buildCmdStr, binaryPath, workspaceDir, suite.Dir, collectPattern)
	if buildCmd != "" {
		compileStart := time.Now()
		cmd := exec.Command("sh", "-c", buildCmd)
		output, err := cmd.CombinedOutput()
		compileTime := time.Since(compileStart)

		if err != nil {
			return &Result{
				ParserSuccess: false,
				BenchmarkMs:   int(compileTime.Milliseconds()),
				Details:       fmt.Sprintf("Build failed:\n%s", string(output)),
			}
		}
	}

	runCmd := expandTemplate(runCmdStr, binaryPath, workspaceDir, suite.Dir, collectPattern)
	if runCmd == "" {
		return &Result{
			ParserSuccess: false,
			Details:       "No run command defined",
		}
	}

	runStart := time.Now()
	cmd := exec.Command("sh", "-c", runCmd)
	output, err := cmd.CombinedOutput()
	runTime := time.Since(runStart)

	benchmarkMs := int(runTime.Milliseconds())
	parserSuccess := err == nil

	details := "Grading Report:\n"
	if parserSuccess {
		details += fmt.Sprintf("-- %s: PASSED\n", suite.Name)
	} else {
		details += fmt.Sprintf("-- %s: FAILED\n", suite.Name)
	}
	details += fmt.Sprintf("-- Benchmark: %d ms (%s)\n", benchmarkMs, RatingFromBenchmark(benchmarkMs))
	details += "\n--- Test Output ---\n"
	details += string(output)

	return &Result{
		ParserSuccess: parserSuccess,
		BenchmarkMs:   benchmarkMs,
		FinalScore:    CalculateScore(parserSuccess, benchmarkMs),
		Details:       details,
	}
}

func runSuiteDocker(suite *Suite, workspaceDir string) *Result {
	lang := DetectLanguage(workspaceDir, suite)
	lc, hasLang := suite.Languages[lang]
	buildCmdStr := suite.Build
	runCmdStr := suite.Run
	collectPattern := "*.c"
	if hasLang {
		if lc.Build != "" {
			buildCmdStr = lc.Build
		}
		if lc.Run != "" {
			runCmdStr = lc.Run
		}
		if lc.Collect != "" {
			collectPattern = lc.Collect
		}
	}

	imageName := "ft-hackthon-suite-" + suite.Name

	if err := buildSuiteImage(imageName, suite.Dir); err != nil {
		return &Result{
			ParserSuccess: false,
			Details:       fmt.Sprintf("Failed to build suite image: %v", err),
		}
	}

	buildCmd := expandDockerTemplate(buildCmdStr, collectPattern)
	runCmd := expandDockerTemplate(runCmdStr, collectPattern)

	fullCmd := buildCmd
	if runCmd != "" {
		if fullCmd != "" {
			fullCmd += " && "
		}
		fullCmd += runCmd
	}
	if fullCmd == "" {
		return &Result{
			ParserSuccess: false,
			Details:       "No build or run command defined",
		}
	}

	timeoutSec, memMB := suite.DefaultTimeoutSeconds, suite.DefaultMemoryMB
	fullCmd = wrapWithTimeout(fullCmd, timeoutSec)

	containerName := "ft-hackthon-grade-" + suite.Name
	cleanupContainer(containerName)

	createArgs := []string{"create", "--name", containerName}
	if memMB > 0 {
		mem := fmt.Sprintf("%dm", memMB)
		createArgs = append(createArgs, "--memory", mem, "--memory-swap", mem)
	}
	createArgs = append(createArgs, imageName, "sh", "-c", fullCmd)

	createOut, err := exec.Command("docker", createArgs...).CombinedOutput()
	if err != nil {
		return &Result{
			ParserSuccess: false,
			Details:       fmt.Sprintf("Failed to create container: %v\n%s", err, string(createOut)),
		}
	}

	cpOut, err := exec.Command("docker", "cp",
		workspaceDir+"/.", containerName+":/workspace/").CombinedOutput()
	if err != nil {
		cleanupContainer(containerName)
		return &Result{
			ParserSuccess: false,
			Details:       fmt.Sprintf("Failed to copy workspace: %v\n%s", err, string(cpOut)),
		}
	}

	runStart := time.Now()
	startOut, err := exec.Command("docker", "start", "-a", containerName).CombinedOutput()
	runTime := time.Since(runStart)

	cleanupContainer(containerName)

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 127 {
			return &Result{
				ParserSuccess: false,
				Details:       fmt.Sprintf("Build/Run failed - command not found in container:\n%s", string(startOut)),
			}
		}
	}

	benchmarkMs := int(runTime.Milliseconds())
	parserSuccess := err == nil

	details := "Grading Report:\n"
	if parserSuccess {
		details += fmt.Sprintf("-- %s: PASSED\n", suite.Name)
	} else {
		details += fmt.Sprintf("-- %s: FAILED\n", suite.Name)
	}
	details += fmt.Sprintf("-- Benchmark: %d ms (%s)\n", benchmarkMs, RatingFromBenchmark(benchmarkMs))
	details += "\n--- Test Output ---\n"
	details += string(startOut)

	return &Result{
		ParserSuccess: parserSuccess,
		BenchmarkMs:   benchmarkMs,
		FinalScore:    CalculateScore(parserSuccess, benchmarkMs),
		Details:       details,
	}
}

func cleanupContainer(name string) {
	if err := exec.Command("docker", "rm", "-f", name).Run(); err != nil {
		log.Printf("Warning: failed to cleanup container %s: %v", name, err)
	}
}

func buildSuiteImage(imageName, suiteDir string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(suiteDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(suiteDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		tw.WriteHeader(header)

		if !fi.IsDir() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			tw.Write(content)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create tar context: %w", err)
	}

	tw.Close()

	cmd := exec.Command("docker", "build", "-q", "-t", imageName, "-")
	cmd.Stdin = &buf
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build: %v\n%s", err, string(out))
	}
	return nil
}

func expandTemplate(tmpl, binary, workspace, suiteDir, pattern string) string {
	r := strings.NewReplacer(
		"{binary}", binary,
		"{workspace}", workspace,
		"{workspace_files}", CollectFiles(workspace, pattern),
		"{suite}", suiteDir,
		"{suite_files}", CollectSuiteFiles(suiteDir),
	)
	return r.Replace(tmpl)
}

func expandDockerTemplate(tmpl, pattern string) string {
	r := strings.NewReplacer(
		"{binary}", "/tmp/binary",
		"{workspace}", "/workspace",
		"{workspace_files}", fmt.Sprintf("/workspace/%s", pattern),
		"{suite}", "/suite",
		"{suite_files}", "/suite/*.c",
	)
	return r.Replace(tmpl)
}

func CollectFiles(dir, pattern string) string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if matched, _ := filepath.Match(pattern, info.Name()); matched && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return strings.Join(files, " ")
}
