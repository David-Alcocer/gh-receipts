package health

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/David-Alcocer/gh-receipts/internal/config"
)

var (
	until  = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	window = Window{Since: until.AddDate(0, 0, -14), Until: until}
)

func hoursBefore(hours float64) time.Time {
	return until.Add(-time.Duration(hours * float64(time.Hour)))
}

// merged returns a pull request merged a day before until, opened mergeHours
// before that.
func merged(number, size int, mergeHours float64) PullRequest {
	mergedAt := hoursBefore(24)
	return PullRequest{
		Number:    number,
		Additions: size,
		CreatedAt: mergedAt.Add(-time.Duration(mergeHours * float64(time.Hour))),
		MergedAt:  mergedAt,
	}
}

func reviewed(pr PullRequest, afterHours float64) PullRequest {
	pr.FirstReviewAt = pr.CreatedAt.Add(time.Duration(afterHours * float64(time.Hour)))
	return pr
}

func median(value float64) *float64 { return &value }

func TestComputePullRequestMetrics(t *testing.T) {
	data := Data{MergedPRs: []PullRequest{
		reviewed(merged(1, 100, 10), 2),
		reviewed(merged(2, 500, 60), 30),
		merged(3, 300, 50),
	}}

	got := Compute(data, window, DefaultThresholds)

	if got.MergedPRs != 3 {
		t.Errorf("merged PRs = %d, want 3", got.MergedPRs)
	}
	assertMetric(t, "size", got.PRSize, Metric{Median: median(300), Count: 3, Limit: 400})
	assertMetric(t, "first review", got.FirstReview, Metric{Median: median(16), Count: 2, Limit: 24})
	assertMetric(t, "merge", got.Merge, Metric{Median: median(50), Count: 3, Limit: 48, OverLimit: true})
}

func TestComputeUsesTheMiddleOfAnEvenCount(t *testing.T) {
	data := Data{MergedPRs: []PullRequest{merged(1, 10, 1), merged(2, 41, 4), merged(3, 20, 2), merged(4, 30, 3)}}

	got := Compute(data, window, DefaultThresholds)

	assertMetric(t, "size", got.PRSize, Metric{Median: median(25), Count: 4, Limit: 400})
	assertMetric(t, "merge", got.Merge, Metric{Median: median(2.5), Count: 4, Limit: 48})
}

func TestComputeCountsAdditionsAndDeletions(t *testing.T) {
	pr := merged(1, 300, 1)
	pr.Deletions = 101

	got := Compute(Data{MergedPRs: []PullRequest{pr}}, window, DefaultThresholds)

	assertMetric(t, "size", got.PRSize, Metric{Median: median(401), Count: 1, Limit: 400, OverLimit: true})
}

func TestComputeIsNotOverTheLimitWhenEqualToIt(t *testing.T) {
	got := Compute(Data{MergedPRs: []PullRequest{merged(1, 400, 48)}}, window, DefaultThresholds)

	if got.PRSize.OverLimit || got.Merge.OverLimit {
		t.Errorf("size = %+v, merge = %+v", got.PRSize, got.Merge)
	}
}

func TestComputeReportsNoDataInsteadOfZero(t *testing.T) {
	tests := []struct {
		name string
		data Data
	}{
		{"nothing merged", Data{}},
		{"no reviews", Data{MergedPRs: []PullRequest{merged(1, 10, 1)}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.data, window, DefaultThresholds)

			if got.FirstReview.Median != nil || got.FirstReview.Count != 0 || got.FirstReview.OverLimit {
				t.Errorf("first review = %+v, want no data", got.FirstReview)
			}
		})
	}
}

func TestComputeIgnoresPullRequestsMergedOutsideTheWindow(t *testing.T) {
	before := merged(1, 10, 1)
	before.MergedAt = window.Since.Add(-time.Minute)
	after := merged(2, 10, 1)
	after.MergedAt = until.Add(time.Minute)
	edge := merged(3, 20, 1)
	edge.MergedAt = window.Since

	got := Compute(Data{MergedPRs: []PullRequest{before, after, edge}}, window, DefaultThresholds)

	if got.MergedPRs != 1 || *got.PRSize.Median != 20 {
		t.Errorf("merged = %d, size = %+v", got.MergedPRs, got.PRSize)
	}
}

func TestComputeAppliesConfiguredThresholds(t *testing.T) {
	thresholds := ThresholdsFrom(config.Health{MaxPRSize: 50, MaxMergeHours: 100})

	got := Compute(Data{MergedPRs: []PullRequest{merged(1, 60, 60)}}, window, thresholds)

	if !got.PRSize.OverLimit || got.Merge.OverLimit || got.Merge.Limit != 100 {
		t.Errorf("size = %+v, merge = %+v", got.PRSize, got.Merge)
	}
	if thresholds.MaxFirstReviewHours != 24 || thresholds.StaleDays != 2 {
		t.Errorf("unset thresholds = %+v, want defaults", thresholds)
	}
}

func TestComputeListsStalePullRequests(t *testing.T) {
	data := Data{OpenPRs: []OpenPullRequest{
		{Number: 1, UpdatedAt: hoursBefore(47)},
		{Number: 2, UpdatedAt: hoursBefore(48)},
		{Number: 3, UpdatedAt: hoursBefore(100)},
		{Number: 4, UpdatedAt: hoursBefore(49)},
		{Number: 5, UpdatedAt: hoursBefore(100)},
	}}

	got := Compute(data, window, DefaultThresholds)

	var numbers, idle []int
	for _, pr := range got.StalePRs {
		numbers = append(numbers, pr.Number)
		idle = append(idle, pr.IdleDays)
	}
	if !slices.Equal(numbers, []int{3, 5, 4}) || !slices.Equal(idle, []int{4, 4, 2}) {
		t.Errorf("stale = %v, idle days = %v", numbers, idle)
	}
}

func TestComputeSplitsIssuesIntoWeeksBackFromUntil(t *testing.T) {
	thisWeek := until.AddDate(0, 0, -1)
	lastWeek := until.AddDate(0, 0, -8)
	data := Data{Issues: []Issue{
		{Number: 1, CreatedAt: thisWeek},
		{Number: 2, CreatedAt: lastWeek, ClosedAt: thisWeek},
		{Number: 3, CreatedAt: window.Since.AddDate(0, 0, -30), ClosedAt: lastWeek},
		{Number: 4, CreatedAt: until.AddDate(0, 0, -7)},
	}}

	got := Compute(data, window, DefaultThresholds)

	want := []IssueWeek{
		{Start: until.AddDate(0, 0, -14), End: until.AddDate(0, 0, -7), Opened: 1, Closed: 1},
		{Start: until.AddDate(0, 0, -7), End: until, Opened: 2, Closed: 1},
	}
	if !slices.Equal(got.IssueWeeks, want) {
		t.Errorf("weeks = %+v\nwant  %+v", got.IssueWeeks, want)
	}
}

func TestComputeShortensTheOldestWeek(t *testing.T) {
	short := Window{Since: until.AddDate(0, 0, -10), Until: until}

	got := Compute(Data{}, short, DefaultThresholds)

	want := []IssueWeek{
		{Start: short.Since, End: until.AddDate(0, 0, -7)},
		{Start: until.AddDate(0, 0, -7), End: until},
	}
	if !slices.Equal(got.IssueWeeks, want) {
		t.Errorf("weeks = %+v\nwant  %+v", got.IssueWeeks, want)
	}
}

func assertMetric(t *testing.T, name string, got, want Metric) {
	t.Helper()
	sameMedian := (got.Median == nil) == (want.Median == nil) && (got.Median == nil || *got.Median == *want.Median)
	if !sameMedian || got.Count != want.Count || got.Limit != want.Limit || got.OverLimit != want.OverLimit {
		t.Errorf("%s = %s, want %s", name, describe(got), describe(want))
	}
}

func describe(m Metric) string {
	median := "nil"
	if m.Median != nil {
		median = fmt.Sprint(*m.Median)
	}
	return fmt.Sprintf("{median %s, count %d, limit %d, over %t}", median, m.Count, m.Limit, m.OverLimit)
}
