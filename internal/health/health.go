// Package health computes team-level flow signals from pull requests and
// issues. Fetch reads the facts and Compute turns them into signals, so the
// calculations can be tested without the network.
package health

import (
	"cmp"
	"slices"
	"time"

	"github.com/David-Alcocer/gh-receipts/internal/config"
)

const week = 7 * 24 * time.Hour

// DefaultThresholds favor small pull requests and short-lived branches, as in
// trunk-based development. docs/health-json.md explains each value.
var DefaultThresholds = Thresholds{MaxPRSize: 400, MaxFirstReviewHours: 24, MaxMergeHours: 48, StaleDays: 2}

// Thresholds mark when a signal is worth a look. Hours are calendar hours.
type Thresholds struct {
	MaxPRSize           int
	MaxFirstReviewHours int
	MaxMergeHours       int
	StaleDays           int
}

// ThresholdsFrom applies the configured thresholds over the defaults.
func ThresholdsFrom(configured config.Health) Thresholds {
	return Thresholds{
		MaxPRSize:           cmp.Or(configured.MaxPRSize, DefaultThresholds.MaxPRSize),
		MaxFirstReviewHours: cmp.Or(configured.MaxFirstReviewHours, DefaultThresholds.MaxFirstReviewHours),
		MaxMergeHours:       cmp.Or(configured.MaxMergeHours, DefaultThresholds.MaxMergeHours),
		StaleDays:           cmp.Or(configured.StaleDays, DefaultThresholds.StaleDays),
	}
}

// Window is the period the signals cover.
type Window struct {
	Since time.Time
	Until time.Time
}

// Data is what Fetch reads. Pull requests and issues authored by bots are
// already excluded.
type Data struct {
	MergedPRs []PullRequest
	OpenPRs   []OpenPullRequest
	Issues    []Issue
	// Warnings describes results that may be incomplete.
	Warnings []string
}

// PullRequest is a merged pull request.
type PullRequest struct {
	Number    int
	URL       string
	Additions int
	Deletions int
	CreatedAt time.Time
	MergedAt  time.Time
	// FirstReviewAt is the first review by someone other than the author or a
	// bot. It is zero when there is none.
	FirstReviewAt time.Time
}

// OpenPullRequest is an open pull request.
type OpenPullRequest struct {
	Number    int
	Title     string
	URL       string
	Draft     bool
	UpdatedAt time.Time
}

// Issue is an issue opened or closed in the window. ClosedAt is zero while it is
// open.
type Issue struct {
	Number    int
	CreatedAt time.Time
	ClosedAt  time.Time
}

// Health holds the signals. It names no one: only counts and medians.
type Health struct {
	Window     Window
	Thresholds Thresholds
	MergedPRs  int
	// PRSize is in lines changed: additions plus deletions.
	PRSize Metric
	// FirstReview and Merge are in hours since the pull request was opened.
	FirstReview Metric
	Merge       Metric
	StalePRs    []StalePullRequest
	IssueWeeks  []IssueWeek
	Warnings    []string
}

// Metric is a median and the number of pull requests it comes from.
type Metric struct {
	// Median is nil when no pull request had the data, which is not the same as
	// zero.
	Median    *float64
	Count     int
	Limit     int
	OverLimit bool
}

// StalePullRequest is an open pull request without activity for more than the
// stale threshold.
type StalePullRequest struct {
	OpenPullRequest
	// IdleDays counts whole days since the last activity.
	IdleDays int
}

// IssueWeek counts the issues opened and closed in [Start, End). Weeks are
// seven-day blocks back from the end of the window, so the oldest one can be
// shorter.
type IssueWeek struct {
	Start  time.Time
	End    time.Time
	Opened int
	Closed int
}

// Compute turns data into signals for the window.
func Compute(data Data, window Window, thresholds Thresholds) Health {
	var sizes, mergeHours, firstReviewHours []float64
	for _, pr := range data.MergedPRs {
		if !within(pr.MergedAt, window) {
			continue
		}
		sizes = append(sizes, float64(pr.Additions+pr.Deletions))
		mergeHours = append(mergeHours, pr.MergedAt.Sub(pr.CreatedAt).Hours())
		if !pr.FirstReviewAt.IsZero() {
			firstReviewHours = append(firstReviewHours, pr.FirstReviewAt.Sub(pr.CreatedAt).Hours())
		}
	}

	return Health{
		Window:      window,
		Thresholds:  thresholds,
		MergedPRs:   len(sizes),
		PRSize:      metric(sizes, thresholds.MaxPRSize),
		FirstReview: metric(firstReviewHours, thresholds.MaxFirstReviewHours),
		Merge:       metric(mergeHours, thresholds.MaxMergeHours),
		StalePRs:    stale(data.OpenPRs, window.Until, thresholds.StaleDays),
		IssueWeeks:  issueWeeks(data.Issues, window),
		Warnings:    data.Warnings,
	}
}

func within(t time.Time, window Window) bool {
	return !t.Before(window.Since) && !t.After(window.Until)
}

func metric(values []float64, limit int) Metric {
	if len(values) == 0 {
		return Metric{Limit: limit}
	}
	m := medianOf(values)
	return Metric{Median: &m, Count: len(values), Limit: limit, OverLimit: m > float64(limit)}
}

func medianOf(values []float64) float64 {
	sorted := slices.Sorted(slices.Values(values))
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// stale returns the pull requests idle for more than staleDays, the longest
// idle first.
func stale(open []OpenPullRequest, now time.Time, staleDays int) []StalePullRequest {
	limit := time.Duration(staleDays) * 24 * time.Hour
	var result []StalePullRequest
	for _, pr := range open {
		idle := now.Sub(pr.UpdatedAt)
		if idle > limit {
			result = append(result, StalePullRequest{OpenPullRequest: pr, IdleDays: int(idle.Hours() / 24)})
		}
	}
	slices.SortFunc(result, func(a, b StalePullRequest) int {
		return cmp.Or(a.UpdatedAt.Compare(b.UpdatedAt), cmp.Compare(a.Number, b.Number))
	})
	return result
}

// issueWeeks counts issues per week, oldest week first.
func issueWeeks(issues []Issue, window Window) []IssueWeek {
	var weeks []IssueWeek
	for end := window.Until; end.After(window.Since); end = end.Add(-week) {
		start := end.Add(-week)
		if start.Before(window.Since) {
			start = window.Since
		}
		w := IssueWeek{Start: start, End: end}
		for _, issue := range issues {
			if inWeek(issue.CreatedAt, w) {
				w.Opened++
			}
			if inWeek(issue.ClosedAt, w) {
				w.Closed++
			}
		}
		weeks = append(weeks, w)
	}
	slices.Reverse(weeks)
	return weeks
}

func inWeek(t time.Time, w IssueWeek) bool {
	return !t.Before(w.Start) && t.Before(w.End)
}
