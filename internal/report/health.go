package report

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/health"
)

// healthSchemaVersion changes when the JSON output changes in an incompatible
// way. docs/health-json.md documents the schema.
const healthSchemaVersion = 1

const weekLayout = "2 Jan"

type healthDocument struct {
	SchemaVersion      int                 `json:"schemaVersion"`
	Repository         string              `json:"repository"`
	Since              time.Time           `json:"since"`
	Until              time.Time           `json:"until"`
	Thresholds         healthThresholds    `json:"thresholds"`
	MergedPullRequests mergedSummary       `json:"mergedPullRequests"`
	PRSize             healthMetric        `json:"prSize"`
	FirstReviewHours   healthMetric        `json:"firstReviewHours"`
	MergeHours         healthMetric        `json:"mergeHours"`
	StalePullRequests  []stalePullRequest  `json:"stalePullRequests"`
	IssueWeeks         []issueWeekDocument `json:"issueWeeks"`
	Warnings           []string            `json:"warnings"`
}

type healthThresholds struct {
	MaxPRSize           int `json:"maxPRSize"`
	MaxFirstReviewHours int `json:"maxFirstReviewHours"`
	MaxMergeHours       int `json:"maxMergeHours"`
	StaleDays           int `json:"staleDays"`
}

type mergedSummary struct {
	Count int    `json:"count"`
	URL   string `json:"url"`
}

type healthMetric struct {
	Median    *float64 `json:"median"`
	Count     int      `json:"count"`
	Limit     int      `json:"limit"`
	OverLimit bool     `json:"overLimit"`
}

type stalePullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Draft     bool      `json:"draft"`
	UpdatedAt time.Time `json:"updatedAt"`
	IdleDays  int       `json:"idleDays"`
}

type issueWeekDocument struct {
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Opened    int       `json:"opened"`
	Closed    int       `json:"closed"`
	OpenedURL string    `json:"openedUrl"`
	ClosedURL string    `json:"closedUrl"`
}

// HealthJSON writes every signal. Unlike HealthText, it never limits sections.
func HealthJSON(w io.Writer, repo repository.Repository, h health.Health) error {
	stale := make([]stalePullRequest, 0, len(h.StalePRs))
	for _, pr := range h.StalePRs {
		stale = append(stale, stalePullRequest{pr.Number, pr.Title, pr.URL, pr.Draft, pr.UpdatedAt, pr.IdleDays})
	}
	weeks := make([]issueWeekDocument, 0, len(h.IssueWeeks))
	for _, week := range h.IssueWeeks {
		weeks = append(weeks, issueWeekDocument{
			Start:     week.Start,
			End:       week.End,
			Opened:    week.Opened,
			Closed:    week.Closed,
			OpenedURL: issueSearchURL(repo, "created", week),
			ClosedURL: issueSearchURL(repo, "closed", week),
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(healthDocument{
		SchemaVersion: healthSchemaVersion,
		Repository:    repo.Owner + "/" + repo.Name,
		Since:         h.Window.Since,
		Until:         h.Window.Until,
		Thresholds: healthThresholds{
			MaxPRSize:           h.Thresholds.MaxPRSize,
			MaxFirstReviewHours: h.Thresholds.MaxFirstReviewHours,
			MaxMergeHours:       h.Thresholds.MaxMergeHours,
			StaleDays:           h.Thresholds.StaleDays,
		},
		MergedPullRequests: mergedSummary{Count: h.MergedPRs, URL: mergedSearchURL(repo, h.Window)},
		PRSize:             metricDocument(h.PRSize),
		FirstReviewHours:   metricDocument(h.FirstReview),
		MergeHours:         metricDocument(h.Merge),
		StalePullRequests:  stale,
		IssueWeeks:         weeks,
		Warnings:           nonNil(h.Warnings),
	})
	if err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

func metricDocument(m health.Metric) healthMetric {
	document := healthMetric{Count: m.Count, Limit: m.Limit, OverLimit: m.OverLimit}
	if m.Median != nil {
		rounded := math.Round(*m.Median*100) / 100
		document.Median = &rounded
	}
	return document
}

// HealthText writes every section, even empty ones: "no data" is an answer.
func HealthText(w io.Writer, repo repository.Repository, h health.Health) error {
	days := int(math.Round(h.Window.Until.Sub(h.Window.Since).Hours() / 24))
	_, err := fmt.Fprintf(w, "Last %d days, %s to %s (calendar hours, bots excluded)\n",
		days, h.Window.Since.Format(timeLayout), h.Window.Until.Format(timeLayout))
	if err != nil {
		return err
	}
	for _, s := range healthSections(repo, h) {
		if err := writeSection(w, s); err != nil {
			return err
		}
	}
	return nil
}

func healthSections(repo repository.Repository, h health.Health) []section {
	merged := mergedSearchURL(repo, h.Window)
	staleBefore := h.Window.Until.Add(-time.Duration(h.Thresholds.StaleDays) * 24 * time.Hour)
	staleURL := repositoryURL(repo) + "/pulls?q=" + url.QueryEscape("is:pr is:open sort:updated-asc updated:<"+staleBefore.UTC().Format(health.SearchTimeLayout))
	return []section{
		{
			title: fmt.Sprintf("PULL REQUESTS: %d merged", h.MergedPRs),
			lines: []line{
				{metricText("Median size", h.PRSize, h.MergedPRs, "lines", formatLines, ""), merged},
				{metricText("Median time to first review", h.FirstReview, h.MergedPRs, "h", formatHours, "a review by someone else"), merged},
				{metricText("Median time to merge", h.Merge, h.MergedPRs, "h", formatHours, ""), merged},
			},
			moreURL: merged,
		},
		{
			title:   fmt.Sprintf("STALE PRS: no activity for more than %d days", h.Thresholds.StaleDays),
			lines:   staleLines(h, staleURL),
			moreURL: staleURL,
		},
		{
			title: "ISSUES PER WEEK",
			lines: issueWeekLines(repo, h.IssueWeeks),
		},
	}
}

// metricText describes a median with where it came from. unit labels the limit,
// format writes the median, and missing names what some pull requests lacked.
func metricText(name string, m health.Metric, merged int, unit string, format func(float64) string, missing string) string {
	if merged == 0 {
		return name + ": no data, no PRs merged"
	}
	if m.Median == nil {
		return fmt.Sprintf("%s: no data, none of %d PRs had %s", name, merged, missing)
	}

	value := format(*m.Median)
	limit := fmt.Sprintf("limit %d %s", m.Limit, unit)
	if m.OverLimit {
		limit = fmt.Sprintf("over the %d %s limit", m.Limit, unit)
	}
	source := fmt.Sprintf("from %d PRs", m.Count)
	if m.Count != merged {
		source = fmt.Sprintf("from %d of %d PRs", m.Count, merged)
	}
	return fmt.Sprintf("%s: %s, %s, %s", name, value, limit, source)
}

func staleLines(h health.Health, staleURL string) []line {
	if len(h.StalePRs) == 0 {
		text := fmt.Sprintf("No open PRs without activity for more than %d days", h.Thresholds.StaleDays)
		return []line{{text, staleURL}}
	}
	lines := make([]line, 0, len(h.StalePRs))
	for _, pr := range h.StalePRs {
		text := fmt.Sprintf("#%d %s  %s idle", pr.Number, clean(pr.Title), plural(pr.IdleDays, "day"))
		if pr.Draft {
			text += ", draft"
		}
		lines = append(lines, line{text, pr.URL})
	}
	return lines
}

func issueWeekLines(repo repository.Repository, weeks []health.IssueWeek) []line {
	lines := make([]line, 0, 2*len(weeks))
	for _, week := range weeks {
		label := week.Start.Format(weekLayout) + " – " + week.End.Format(weekLayout)
		lines = append(lines,
			line{fmt.Sprintf("%s  %d opened", label, week.Opened), issueSearchURL(repo, "created", week)},
			line{fmt.Sprintf("%s  %d closed", label, week.Closed), issueSearchURL(repo, "closed", week)},
		)
	}
	return lines
}

func mergedSearchURL(repo repository.Repository, window health.Window) string {
	return repositoryURL(repo) + "/pulls?q=" + url.QueryEscape("is:pr is:merged merged:>="+window.Since.UTC().Format(health.SearchTimeLayout))
}

// issueSearchURL lists the issues of a week. GitHub ranges include both ends,
// so an issue at the exact second a week ends appears in both weeks' links.
func issueSearchURL(repo repository.Repository, qualifier string, week health.IssueWeek) string {
	span := week.Start.UTC().Format(health.SearchTimeLayout) + ".." + week.End.UTC().Format(health.SearchTimeLayout)
	return repositoryURL(repo) + "/issues?q=" + url.QueryEscape("is:issue "+qualifier+":"+span)
}

func formatLines(value float64) string {
	return formatTenth(value) + " lines"
}

// formatHours shows minutes under an hour, where tenths of an hour would round
// quick reviews down to 0.
func formatHours(value float64) string {
	if minutes := math.Round(value * 60); minutes < 60 {
		return fmt.Sprintf("%d min", int(minutes))
	}
	return formatTenth(value) + " h"
}

func formatTenth(value float64) string {
	return strconv.FormatFloat(math.Round(value*10)/10, 'f', -1, 64)
}

func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
