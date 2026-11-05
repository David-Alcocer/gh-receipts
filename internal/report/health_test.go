package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/David-Alcocer/gh-receipts/internal/health"
)

var healthUntil = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func sampleHealth() health.Health {
	since := healthUntil.AddDate(0, 0, -14)
	size, review, merge := 150.0, 6.04, 49.25
	return health.Health{
		Window:      health.Window{Since: since, Until: healthUntil},
		Thresholds:  health.DefaultThresholds,
		MergedPRs:   3,
		PRSize:      health.Metric{Median: &size, Count: 3, Limit: 400},
		FirstReview: health.Metric{Median: &review, Count: 2, Limit: 24},
		Merge:       health.Metric{Median: &merge, Count: 3, Limit: 48, OverLimit: true},
		StalePRs: []health.StalePullRequest{
			{OpenPullRequest: health.OpenPullRequest{Number: 20, Title: "Add\texport", URL: "https://github.com/o/r/pull/20", UpdatedAt: healthUntil.AddDate(0, 0, -6)}, IdleDays: 6},
			{OpenPullRequest: health.OpenPullRequest{Number: 22, Title: "Try new layout", URL: "https://github.com/o/r/pull/22", Draft: true, UpdatedAt: healthUntil.AddDate(0, 0, -3)}, IdleDays: 3},
		},
		IssueWeeks: []health.IssueWeek{
			{Start: since, End: healthUntil.AddDate(0, 0, -7), Opened: 2, Closed: 1},
			{Start: healthUntil.AddDate(0, 0, -7), End: healthUntil, Opened: 1, Closed: 0},
		},
	}
}

func emptyHealth() health.Health {
	return health.Health{
		Window:      health.Window{Since: healthUntil.AddDate(0, 0, -7), Until: healthUntil},
		Thresholds:  health.DefaultThresholds,
		PRSize:      health.Metric{Limit: 400},
		FirstReview: health.Metric{Limit: 24},
		Merge:       health.Metric{Limit: 48},
		IssueWeeks:  []health.IssueWeek{{Start: healthUntil.AddDate(0, 0, -7), End: healthUntil}},
	}
}

func renderHealth(t *testing.T, h health.Health) string {
	t.Helper()
	var out bytes.Buffer
	if err := HealthText(&out, testRepo, h); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out.String()
}

func TestHealthTextGolden(t *testing.T) {
	assertGolden(t, "health.golden", renderHealth(t, sampleHealth()))
}

func TestHealthTextSaysNoDataInsteadOfZero(t *testing.T) {
	got := renderHealth(t, emptyHealth())

	for _, want := range []string{
		"Median size: no data, no PRs merged",
		"Median time to first review: no data, no PRs merged",
		"No open PRs without activity for more than 2 days",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("text is missing %q:\n%s", want, got)
		}
	}
}

func TestHealthTextExplainsMissingReviews(t *testing.T) {
	h := emptyHealth()
	h.MergedPRs = 2

	got := renderHealth(t, h)

	if !strings.Contains(got, "Median time to first review: no data, none of 2 PRs had a review by someone else") {
		t.Errorf("text =\n%s", got)
	}
}

func TestHealthTextShowsMinutesUnderAnHour(t *testing.T) {
	h := sampleHealth()
	review, merge := 0.03, 0.999
	h.FirstReview.Median, h.Merge.Median = &review, &merge
	h.Merge.OverLimit = false

	got := renderHealth(t, h)

	for _, want := range []string{"first review: 2 min, limit 24 h", "merge: 1 h, limit 48 h"} {
		if !strings.Contains(got, want) {
			t.Errorf("text is missing %q:\n%s", want, got)
		}
	}
}

func TestHealthJSONKeepsShortTimes(t *testing.T) {
	h := sampleHealth()
	review := 0.03
	h.FirstReview.Median = &review
	var out bytes.Buffer

	if err := HealthJSON(&out, testRepo, h); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), `"median": 0.03,`) {
		t.Errorf("JSON =\n%s", out.String())
	}
}

func TestHealthTextEveryFactEndsWithAURL(t *testing.T) {
	for _, h := range []health.Health{sampleHealth(), emptyHealth()} {
		lines := strings.Split(renderHealth(t, h), "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, "  ") {
				continue
			}
			if !strings.Contains(line, "https://github.com/o/r/") {
				t.Errorf("line without a URL: %q", line)
			}
		}
	}
}

func TestHealthJSONGolden(t *testing.T) {
	var out bytes.Buffer
	if err := HealthJSON(&out, testRepo, sampleHealth()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertGolden(t, "health.json.golden", out.String())
}

func TestHealthJSONUsesNullForNoData(t *testing.T) {
	var out bytes.Buffer
	if err := HealthJSON(&out, testRepo, emptyHealth()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	var size struct {
		Median *float64 `json:"median"`
		Count  int      `json:"count"`
	}
	if err := json.Unmarshal(document["prSize"], &size); err != nil {
		t.Fatal(err)
	}
	if size.Median != nil || size.Count != 0 {
		t.Errorf("prSize = %s", document["prSize"])
	}
	for _, list := range []string{"stalePullRequests", "warnings"} {
		if string(document[list]) != "[]" {
			t.Errorf("%s = %s, want []", list, document[list])
		}
	}
}
