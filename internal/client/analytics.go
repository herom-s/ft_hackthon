package client

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ChallengeStats struct {
	Name      string
	Title     string
	TotalRuns int
	Passed    int
	Failed    int
	BestScore int
	AvgScore  float64
}

type ReportOptions struct {
	ChallengeFilter string
	DaysBack        int
	ShowTrend       bool
}

func (sm *SubmitManager) GenerateReport(opts ReportOptions) error {
	jobs, err := sm.apiClient.ListJobs()
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	if len(jobs.Jobs) == 0 {
		fmt.Println("No submissions yet.")
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -opts.DaysBack)

	challengeStats := make(map[string]*ChallengeStats)
	type dailyEntry struct {
		Date  string
		Score int
	}
	trendData := make(map[string][]dailyEntry)

	for _, j := range jobs.Jobs {
		t, err := time.Parse(time.RFC3339, j.CreatedAt)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			continue
		}

		if j.Result == nil || len(j.Result.Challenges) == 0 {
			continue
		}

		for _, ch := range j.Result.Challenges {
			if opts.ChallengeFilter != "" && !strings.EqualFold(ch.Name, opts.ChallengeFilter) {
				continue
			}

			stats, ok := challengeStats[ch.Title]
			if !ok {
				stats = &ChallengeStats{Name: ch.Name, Title: ch.Title}
				challengeStats[ch.Title] = stats
			}

			stats.TotalRuns++
			if ch.Passed {
				stats.Passed++
				if ch.Points > stats.BestScore {
					stats.BestScore = ch.Points
				}
			} else {
				stats.Failed++
			}

		stats.AvgScore += float64(ch.Points)

			if opts.ShowTrend {
				dateKey := t.Format("2006-01-02")
				trendData[ch.Title] = append(trendData[ch.Title], dailyEntry{
					Date:  dateKey,
					Score: ch.Points,
				})
			}
		}
	}

	if len(challengeStats) == 0 {
		fmt.Println("No matching submissions found.")
		return nil
	}

	titles := make([]string, 0, len(challengeStats))
	for title := range challengeStats {
		titles = append(titles, title)
	}
	sort.Strings(titles)

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("  SUBMISSION ANALYTICS")
	fmt.Println("==========================================")
	fmt.Println()

	for _, title := range titles {
		stats := challengeStats[title]
		if stats.TotalRuns > 0 {
			stats.AvgScore /= float64(stats.TotalRuns)
		}

		passRate := float64(stats.Passed) / float64(stats.TotalRuns) * 100

		fmt.Printf("  %s\n", title)
		fmt.Printf("    Submissions: %d  |  Pass: %d  Fail: %d  (%.0f%% pass rate)\n",
			stats.TotalRuns, stats.Passed, stats.Failed, passRate)
		fmt.Printf("    Best score:  %d pts\n", stats.BestScore)
		fmt.Printf("    Avg score:   %.1f pts\n", stats.AvgScore)

		if opts.ShowTrend && len(trendData[title]) > 0 {
			fmt.Println("    Trend:")
			entries := trendData[title]
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Date < entries[j].Date
			})
			for _, e := range entries {
				bar := strings.Repeat("#", e.Score/10)
				fmt.Printf("      %s |%s %d\n", e.Date, bar, e.Score)
			}
		}
		fmt.Println()
	}

	return nil
}
