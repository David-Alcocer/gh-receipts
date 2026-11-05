package health

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
)

var testRepo = repository.Repository{Host: "github.com", Owner: "o", Name: "r"}

// fakeClient answers from fixtures, keyed by operation name and cursor. The two
// issue searches are told apart by their qualifier.
type fakeClient struct {
	t        *testing.T
	fixtures map[string]string
	searches []string
	failOn   string
	err      error
}

func (c *fakeClient) DoWithContext(_ context.Context, query string, variables map[string]interface{}, response interface{}) error {
	c.t.Helper()
	operation := strings.SplitN(strings.Fields(query)[1], "(", 2)[0]
	if operation == c.failOn {
		return c.err
	}
	key := operation
	if search, ok := variables["search"].(string); ok {
		c.searches = append(c.searches, search)
		if operation == "Issues" {
			key += " " + strings.SplitN(strings.Fields(search)[2], ":", 2)[0]
		}
	}

	cursor, _ := variables["cursor"].(string)
	name, ok := c.fixtures[key+"|"+cursor]
	if !ok {
		c.t.Fatalf("unexpected request %s with cursor %q", key, cursor)
	}
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		c.t.Fatal(err)
	}
	var envelope struct{ Data json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		c.t.Fatal(err)
	}
	return json.Unmarshal(envelope.Data, response)
}

func syntheticClient(t *testing.T) *fakeClient {
	t.Helper()
	return &fakeClient{t: t, fixtures: map[string]string{
		"RepositoryExists|":     "synthetic_repository.json",
		"MergedPullRequests|":   "synthetic_merged_page1.json",
		"MergedPullRequests|M1": "synthetic_merged_page2.json",
		"StalePullRequests|":    "synthetic_stale.json",
		"Issues created|":       "synthetic_issues_created.json",
		"Issues closed|":        "synthetic_issues_closed.json",
	}}
}

func fetchSynthetic(t *testing.T) (Data, *fakeClient) {
	t.Helper()
	client := syntheticClient(t)
	got, err := FetchWith(context.Background(), client, testRepo, window, DefaultThresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return got, client
}

func at(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestFetchSearchesTheWindow(t *testing.T) {
	_, client := fetchSynthetic(t)

	want := []string{
		"repo:o/r is:pr is:merged merged:>=2026-10-21T12:00:00Z",
		"repo:o/r is:pr is:merged merged:>=2026-10-21T12:00:00Z",
		"repo:o/r is:pr is:open updated:<2026-11-02T12:00:00Z",
		"repo:o/r is:issue created:>=2026-10-21T12:00:00Z",
		"repo:o/r is:issue closed:>=2026-10-21T12:00:00Z",
	}
	if !slices.Equal(client.searches, want) {
		t.Errorf("searches = %q\nwant       %q", client.searches, want)
	}
}

func TestFetchReadsMergedPullRequestsAcrossPages(t *testing.T) {
	got, _ := fetchSynthetic(t)

	var numbers []int
	for _, pr := range got.MergedPRs {
		numbers = append(numbers, pr.Number)
	}
	if !slices.Equal(numbers, []int{10, 12, 13}) {
		t.Errorf("merged = %v, want bots excluded", numbers)
	}
	first := got.MergedPRs[0]
	if first.Additions != 120 || first.Deletions != 30 || !first.MergedAt.Equal(at(t, "2026-10-31T09:00:00Z")) {
		t.Errorf("first = %+v", first)
	}
}

func TestFetchFindsTheFirstReviewBySomeoneElse(t *testing.T) {
	got, _ := fetchSynthetic(t)

	want := map[int]time.Time{
		// Skips the author, a bot and a pending review.
		10: at(t, "2026-10-30T15:00:00Z"),
		// A deleted account is someone else.
		12: at(t, "2026-11-01T12:00:00Z"),
		// Only the author reviewed.
		13: {},
	}
	for _, pr := range got.MergedPRs {
		if !pr.FirstReviewAt.Equal(want[pr.Number]) {
			t.Errorf("#%d first review = %v, want %v", pr.Number, pr.FirstReviewAt, want[pr.Number])
		}
	}
}

func TestFetchWarnsWhenReviewsWereCut(t *testing.T) {
	got, _ := fetchSynthetic(t)

	want := []string{"only the first 50 reviews of https://github.com/o/r/pull/13 were read"}
	if !slices.Equal(got.Warnings, want) {
		t.Errorf("warnings = %q", got.Warnings)
	}
}

func TestFetchReadsOpenPullRequestsWithoutBots(t *testing.T) {
	got, _ := fetchSynthetic(t)

	want := []OpenPullRequest{
		{Number: 20, Title: "Add export", URL: "https://github.com/o/r/pull/20", UpdatedAt: at(t, "2026-10-29T08:00:00Z")},
		{Number: 22, Title: "Try new layout", URL: "https://github.com/o/r/pull/22", Draft: true, UpdatedAt: at(t, "2026-11-01T08:00:00Z")},
	}
	if !slices.Equal(got.OpenPRs, want) {
		t.Errorf("open = %+v\nwant   %+v", got.OpenPRs, want)
	}
}

func TestFetchKeepsEachIssueOnce(t *testing.T) {
	got, _ := fetchSynthetic(t)

	want := []Issue{
		{Number: 30, CreatedAt: at(t, "2026-09-01T08:00:00Z"), ClosedAt: at(t, "2026-10-28T08:00:00Z")},
		{Number: 31, CreatedAt: at(t, "2026-10-25T08:00:00Z"), ClosedAt: at(t, "2026-11-02T08:00:00Z")},
		{Number: 32, CreatedAt: at(t, "2026-11-03T08:00:00Z")},
	}
	if !slices.Equal(got.Issues, want) {
		t.Errorf("issues = %+v\nwant     %+v", got.Issues, want)
	}
}

func TestFetchWrapsErrors(t *testing.T) {
	cause := errors.New("HTTP 502")
	for _, operation := range []string{"RepositoryExists", "MergedPullRequests", "StalePullRequests", "Issues"} {
		t.Run(operation, func(t *testing.T) {
			client := syntheticClient(t)
			client.failOn, client.err = operation, cause

			_, err := FetchWith(context.Background(), client, testRepo, window, DefaultThresholds)

			if !errors.Is(err, cause) {
				t.Errorf("error = %v, want it to wrap %v", err, cause)
			}
		})
	}
}

func TestFetchStopsOnAMissingRepository(t *testing.T) {
	client := syntheticClient(t)
	client.failOn, client.err = "RepositoryExists", errors.New("Could not resolve to a Repository")

	_, err := FetchWith(context.Background(), client, testRepo, window, DefaultThresholds)

	if err == nil || !strings.Contains(err.Error(), "o/r") || len(client.searches) != 0 {
		t.Errorf("error = %v, searches = %q", err, client.searches)
	}
}
