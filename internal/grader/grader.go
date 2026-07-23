package grader

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const workspaceConfigFile = "ft_hackthon.yml"

// ComputeChecksum computes SHA256 of all files matching pattern under dir.
func ComputeChecksum(dir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type Suite struct {
	Name                  string                    `yaml:"name"`
	Language              string                    `yaml:"language"`
	Languages             map[string]LanguageConfig `yaml:"languages,omitempty"`
	Detect                []string                  `yaml:"detect"`
	Build                 string                    `yaml:"build"`
	Run                   string                    `yaml:"run"`
	StartsAt              string                    `yaml:"starts_at,omitempty"`
	EndsAt                string                    `yaml:"ends_at,omitempty"`
	DefaultTimeoutSeconds int                       `yaml:"default_timeout_seconds,omitempty"`
	DefaultMemoryMB       int                       `yaml:"default_memory_mb,omitempty"`
	Dir                   string                    `yaml:"-"`
}

type LanguageConfig struct {
	Build      string `yaml:"build"`
	Run        string `yaml:"run"`
	Extension  string `yaml:"extension"`
	Collect    string `yaml:"collect"`
	TestRunner string `yaml:"test_runner"`
}

type Challenge struct {
	Name           string `yaml:"name"`
	Title          string `yaml:"title"`
	Points         int    `yaml:"points"`
	TargetDir      string `yaml:"target_dir,omitempty"`
	TimeoutSeconds int    `yaml:"timeout_seconds,omitempty"`
	MemoryMB       int    `yaml:"memory_mb,omitempty"`
	Dir            string `yaml:"-"`
}

type ChallengeResult struct {
	Name        string
	Title       string
	Passed      bool
	Points      int
	TestsRun    int
	TestsPassed int
	BenchmarkMs int
	Details     string
}

type Result struct {
	ParserSuccess bool
	BenchmarkMs   int
	FinalScore    int
	Details       string
	Challenges    []ChallengeResult
	CodeChecksum  string
}

type WorkspaceConfig struct {
	Suite string `yaml:"suite"`
}

func LoadWorkspaceConfig(workspaceDir string) (*WorkspaceConfig, error) {
	path := filepath.Join(workspaceDir, workspaceConfigFile)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg WorkspaceConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", workspaceConfigFile, err)
	}
	return &cfg, nil
}

func SaveWorkspaceConfig(workspaceDir, suiteName string) error {
	cfg := WorkspaceConfig{Suite: suiteName}
	path := filepath.Join(workspaceDir, workspaceConfigFile)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	defer enc.Close()
	return enc.Encode(cfg)
}

func LoadSuiteByName(name string) *Suite {
	if suitesPath == "" {
		return nil
	}
	suite, err := LoadSuite(filepath.Join(suitesPath, name))
	if err != nil {
		return nil
	}
	return suite
}

func LoadChallenge(dir string) (*Challenge, error) {
	path := filepath.Join(dir, "challenge.yml")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var c Challenge
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("invalid challenge.yml in %s: %w", dir, err)
	}
	c.Dir = dir
	return &c, nil
}

func LoadChallenges(suite *Suite) []*Challenge {
	challengesDir := filepath.Join(suite.Dir, "challenges")
	entries, err := os.ReadDir(challengesDir)
	if err != nil {
		return nil
	}
	var challenges []*Challenge
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := LoadChallenge(filepath.Join(challengesDir, e.Name()))
		if err != nil {
			continue
		}
		challenges = append(challenges, c)
	}
	return challenges
}

func DetectSuite(workspaceDir string) *Suite {
	if suitesPath == "" {
		return nil
	}

	entries, err := os.ReadDir(suitesPath)
	if err != nil {
		return nil
	}

	var best *Suite
	bestScore := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		suite, err := LoadSuite(filepath.Join(suitesPath, entry.Name()))
		if err != nil {
			continue
		}
		if suite.Matches(workspaceDir) {
			score := len(suite.Detect)
			if score > bestScore {
				best = suite
				bestScore = score
			}
		}
	}
	return best
}

func (s *Suite) IsActive(now time.Time) (bool, string) {
	if s.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, s.StartsAt)
		if err == nil && now.Before(t) {
			return false, fmt.Sprintf("Starts at %s", t.Format("Jan 02 15:04 MST"))
		}
	}
	if s.EndsAt != "" {
		t, err := time.Parse(time.RFC3339, s.EndsAt)
		if err == nil && now.After(t) {
			return false, fmt.Sprintf("Ended at %s", t.Format("Jan 02 15:04 MST"))
		}
	}
	return true, ""
}

func (s *Suite) Matches(workspaceDir string) bool {
	for _, f := range s.Detect {
		path := filepath.Join(workspaceDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return false
		}
	}
	return len(s.Detect) > 0
}

// DetectLanguage returns which language key to use for grading.
// Checks workspace files against configured language extensions.
// Falls back to suite-level "language" field if no match.
func DetectLanguage(workspaceDir string, suite *Suite) string {
	if len(suite.Languages) == 0 {
		return suite.Language
	}

	extCount := make(map[string]int)
	filepath.Walk(workspaceDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != "" {
			extCount[ext]++
		}
		return nil
	})

	bestLang := suite.Language
	bestCount := 0
	for key, lc := range suite.Languages {
		if lc.Extension != "" {
			count := extCount[lc.Extension]
			if count > bestCount {
				bestCount = count
				bestLang = key
			}
		}
	}
	return bestLang
}

func ListSuites() []string {
	if suitesPath == "" {
		return nil
	}
	entries, err := os.ReadDir(suitesPath)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := LoadSuite(filepath.Join(suitesPath, e.Name())); err == nil {
				names = append(names, e.Name())
			}
		}
	}
	return names
}

var suitesPath string

func SetSuitesPath(path string) {
	suitesPath = path
}

func SuitesPath() string {
	return suitesPath
}

func LoadSuite(dir string) (*Suite, error) {
	path := filepath.Join(dir, "suite.yml")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var s Suite
	if err := yaml.NewDecoder(f).Decode(&s); err != nil {
		return nil, fmt.Errorf("invalid suite.yml in %s: %w", dir, err)
	}
	s.Dir = dir
	return &s, nil
}

func CollectSuiteFiles(dir string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.c"))
	return strings.Join(matches, " ")
}

func CalculateScore(parserSuccess bool, benchmarkMs int) int {
	if !parserSuccess {
		return 0
	}
	score := 50
	switch {
	case benchmarkMs <= 100:
		score += 50
	case benchmarkMs <= 150:
		score += 40
	case benchmarkMs <= 200:
		score += 30
	case benchmarkMs <= 300:
		score += 20
	default:
		score += 10
	}
	return score
}

func RatingFromBenchmark(ms int) string {
	switch {
	case ms <= 100:
		return "Excellent"
	case ms <= 150:
		return "Very Good"
	case ms <= 200:
		return "Good"
	case ms <= 300:
		return "Acceptable"
	default:
		return "Needs Optimization"
	}
}

const (
	DefaultEloRating = 1200
	eloKFactor       = 32
)

// ComputeNewRating calculates the new Elo rating based on the user's score.
// Higher scores increase rating, lower scores decrease it.
// A user at DefaultEloRating needs 50% to stay even.
// K-factor = 32, minimum rating = 100.
func ComputeNewRating(currentRating, userScore int) int {
	// Normalized performance (0.0 to 1.0)
	performance := float64(userScore) / 100.0

	// Expected score: 50% at 1200, higher-rated users need higher scores
	expected := 1.0 / (1.0 + math.Pow(10, float64(DefaultEloRating-currentRating)/400.0))

	newRating := currentRating + int(math.Round(eloKFactor*(performance-expected)))
	if newRating < 100 {
		newRating = 100
	}
	return newRating
}
