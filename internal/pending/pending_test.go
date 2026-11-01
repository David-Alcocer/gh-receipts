package pending

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

var testRepo = repository.Repository{Host: "github.com", Owner: "o", Name: "r"}

// fakeClient answers from fixtures, keyed by operation name and cursor.
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
	if search, ok := variables["search"].(string); ok {
		c.searches = append(c.searches, search)
	}

	cursor, _ := variables["cursor"].(string)
	name, ok := c.fixtures[operation+"|"+cursor]
	if !ok {
		c.t.Fatalf("unexpected request %s with cursor %q", operation, cursor)
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

func recordedClient(t *testing.T) *fakeClient {
	t.Helper()
	return &fakeClient{t: t, fixtures: map[string]string{
		"RepositoryExists|":             "synthetic_repository.json",
		"ReviewRequests|":               "synthetic_review_requests.json",
		"OpenPullRequests|":             "pull_requests_cli.json",
		"OpenPullRequests|Y3Vyc29yOjI1": "synthetic_pull_requests_last.json",
		"AssignedIssues|":               "issues_receipts.json",
	}}
}

func fetchRecorded(t *testing.T) (Pending, *fakeClient) {
	t.Helper()
	client := recordedClient(t)
	got, err := FetchWith(context.Background(), client, testRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return got, client
}

func TestFetchSearchesOnlyTheViewersOpenWork(t *testing.T) {
	_, client := fetchRecorded(t)

	want := map[string]bool{
		"repo:o/r is:pr is:open review-requested:@me": false,
		"repo:o/r is:pr is:open author:@me":           false,
		"repo:o/r is:issue is:open assignee:@me":      false,
	}
	for _, search := range client.searches {
		if _, ok := want[search]; !ok {
			t.Errorf("unexpected search %q", search)
		}
		want[search] = true
	}
	for search, seen := range want {
		if !seen {
			t.Errorf("search %q never ran", search)
		}
	}
}

func TestFetchRecordedPullRequestsAcrossPages(t *testing.T) {
	got, _ := fetchRecorded(t)

	if got.Viewer != "octocat" {
		t.Errorf("viewer = %q", got.Viewer)
	}
	if len(got.PullRequests) != 26 {
		t.Fatalf("pull requests = %d, want 26", len(got.PullRequests))
	}
	byNumber := map[int]PullRequest{}
	for _, pr := range got.PullRequests {
		byNumber[pr.Number] = pr
	}
	draft := byNumber[14781]
	if !draft.Draft || draft.CIState != "SUCCESS" || draft.ReviewDecision != "REVIEW_REQUIRED" || draft.URL != "https://github.com/cli/cli/pull/14781" {
		t.Errorf("draft = %+v", draft)
	}
	if noChecks := byNumber[10423]; noChecks.Draft || noChecks.CIState != "" {
		t.Errorf("without checks = %+v", noChecks)
	}
	if last := byNumber[1]; last.ReviewDecision != "" || last.CIState != "" {
		t.Errorf("without commits or decision = %+v", last)
	}
}

func TestFetchSortsPullRequests(t *testing.T) {
	got, _ := fetchRecorded(t)

	// One recorded pull request failed its checks, so it goes first.
	if first := got.PullRequests[0]; first.CIState != "FAILURE" {
		t.Errorf("first = %+v", first)
	}
	rest := got.PullRequests[1:]
	if !slices.IsSortedFunc(rest, func(a, b PullRequest) int { return a.Number - b.Number }) {
		t.Errorf("not sorted by number: %+v", rest)
	}
}

func TestFetchFailsForAMissingRepository(t *testing.T) {
	client := recordedClient(t)
	client.failOn, client.err = "RepositoryExists", errors.New("Could not resolve to a Repository")

	got, err := FetchWith(context.Background(), client, testRepo)

	if err == nil || !strings.Contains(err.Error(), "read repository o/r") {
		t.Errorf("error = %v, pending = %+v", err, got)
	}
	if len(client.searches) != 0 {
		t.Errorf("searches ran anyway: %v", client.searches)
	}
}

func TestFetchRecordedIssuesWithCriteria(t *testing.T) {
	got, _ := fetchRecorded(t)

	if len(got.Issues) != 3 {
		t.Fatalf("issues = %+v", got.Issues)
	}
	for i, number := range []int{6, 12, 14} {
		if got.Issues[i].Number != number {
			t.Errorf("issue %d = %d, want %d", i, got.Issues[i].Number, number)
		}
	}
	if issue := got.Issues[2]; !issue.Criteria.Found || len(issue.Criteria.Items) != 2 || issue.URL != "https://github.com/David-Alcocer/gh-receipts/issues/14" {
		t.Errorf("issue 14 = %+v", issue)
	}
	if nested := got.Issues[0].Criteria.Items[1]; nested.Depth != 1 {
		t.Errorf("nested item = %+v", nested)
	}
}

func TestFetchReviewRequestsOldestFirstWithoutBots(t *testing.T) {
	got, _ := fetchRecorded(t)

	if len(got.ReviewRequests) != 2 || got.ReviewRequests[0].Number != 3 || got.ReviewRequests[1].Number != 9 {
		t.Errorf("review requests = %+v", got.ReviewRequests)
	}
}

func TestPullRequestsThatNeedActionComeFirst(t *testing.T) {
	prs := []PullRequest{
		{Number: 2, CIState: "PENDING"},
		{Number: 5, CIState: "FAILURE"},
		{Number: 1, CIState: "SUCCESS", ReviewDecision: "APPROVED"},
		{Number: 4, CIState: "ERROR"},
		{Number: 3, CIState: "SUCCESS", ReviewDecision: "CHANGES_REQUESTED"},
	}

	sortPullRequests(prs)

	var got []int
	for _, pr := range prs {
		got = append(got, pr.Number)
	}
	want := []int{3, 4, 5, 1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestFetchKeepsTheSourceOfErrors(t *testing.T) {
	cause := errors.New("boom")
	for _, operation := range []string{"RepositoryExists", "ReviewRequests", "OpenPullRequests", "AssignedIssues"} {
		client := recordedClient(t)
		client.failOn, client.err = operation, cause

		_, err := FetchWith(context.Background(), client, testRepo)

		if !errors.Is(err, cause) {
			t.Errorf("%s: error = %v", operation, err)
		}
	}
}

func TestFetchWarnsAboutTruncatedSearches(t *testing.T) {
	client := recordedClient(t)
	client.fixtures["AssignedIssues|"] = "synthetic_issues_truncated.json"

	got, err := FetchWith(context.Background(), client, testRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "matched 1500 results") {
		t.Errorf("warnings = %v", got.Warnings)
	}
}
