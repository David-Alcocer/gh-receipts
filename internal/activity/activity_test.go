package activity

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

// fakeClient answers queries from fixture files, keyed by query, node ID and
// cursor. A request with no fixture fails the test.
type fakeClient struct {
	t        *testing.T
	fixtures map[string]string
	searches []string
	err      error
}

func requestKey(query, id, cursor string) string {
	return query + "|" + id + "|" + cursor
}

func (c *fakeClient) DoWithContext(_ context.Context, query string, variables map[string]interface{}, response interface{}) error {
	c.t.Helper()
	if c.err != nil {
		return c.err
	}
	id, _ := variables["id"].(string)
	cursor, _ := variables["cursor"].(string)
	if search, ok := variables["search"].(string); ok {
		c.searches = append(c.searches, search)
	}

	fixture, ok := c.fixtures[requestKey(query, id, cursor)]
	if !ok {
		c.t.Fatalf("unexpected request: id=%q cursor=%q query=%.40q", id, cursor, query)
	}
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		c.t.Fatal(err)
	}
	var envelope struct{ Data json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		c.t.Fatal(err)
	}
	return json.Unmarshal(envelope.Data, response)
}

func date(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func prNumbers(prs []PullRequest) []int {
	numbers := make([]int, 0, len(prs))
	for _, pr := range prs {
		numbers = append(numbers, pr.Number)
	}
	slices.Sort(numbers)
	return numbers
}

func TestFetchActivityFromRecordedSearch(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{
		requestKey(activityQuery, "", ""):       "activity_cli.json",
		requestKey(reviewRequestsQuery, "", ""): "review_requests_empty.json",
		requestKey(checksQuery, "", ""):         "checks_passing.json",
	}}

	got, err := FetchWith(context.Background(), client, testRepo, date(t, "2026-10-30T16:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := []int{14348, 14349, 14350, 14351, 14355, 14879}; !slices.Equal(prNumbers(got.MergedPRs), want) {
		t.Errorf("merged = %v, want %v", prNumbers(got.MergedPRs), want)
	}
	if want := []int{14879}; !slices.Equal(prNumbers(got.OpenedPRs), want) {
		t.Errorf("opened = %v, want %v", prNumbers(got.OpenedPRs), want)
	}
	if len(got.Reviews) != 6 {
		t.Errorf("reviews = %d, want 6", len(got.Reviews))
	}
	if len(got.Comments) != 7 {
		t.Errorf("comments = %d, want 7", len(got.Comments))
	}
	if len(got.FailedChecks) != 0 {
		t.Errorf("failed checks = %v, want none", got.FailedChecks)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", got.Warnings)
	}
}

func TestFetchSkipsHistoryCommitsWithoutChecks(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{requestKey(checksQuery, "", ""): "checks_passing.json"}}

	got, err := fetchFailedChecks(context.Background(), client, testRepo, date(t, "2026-10-27T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Failed) != 0 {
		t.Errorf("failed checks = %v, want none", got.Failed)
	}
}

func TestFetchFailedChecksIgnoresCancelledRuns(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{requestKey(checksQuery, "", ""): "checks_failed.json"}}

	got, err := fetchFailedChecks(context.Background(), client, testRepo, date(t, "2026-10-30T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []FailedCheck{{
		Name:       "lint-cpp",
		Conclusion: "FAILURE",
		CommitSHA:  "ddb055941b1901755ef33c208118815b3c586acb",
		URL:        "https://github.com/nodejs/node/runs/124204094871",
	}}
	if !slices.Equal(got.Failed, want) {
		t.Errorf("failed checks = %+v, want %+v", got.Failed, want)
	}
}

func TestFetchReadsInlineReviewComments(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{requestKey(activityQuery, "", ""): "activity_inline.json"}}

	got, err := fetchUpdated(context.Background(), client, testRepo, date(t, "2026-10-29T10:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var inline *Comment
	for i := range got.Comments {
		if strings.HasSuffix(got.Comments[i].URL, "#discussion_r4417161557") {
			inline = &got.Comments[i]
		}
	}
	if inline == nil {
		t.Fatal("inline comment r4417161557 is missing")
	}
	if inline.Author != "williammartin" || inline.ParentNumber != 14355 || inline.ParentAuthor != "williammartin" {
		t.Errorf("unexpected inline comment: %+v", *inline)
	}
	if inline.ReviewURL == "" {
		t.Error("inline comment should link to its review")
	}
	for _, comment := range got.Comments {
		if comment.Author == "copilot-pull-request-reviewer" {
			t.Errorf("bot comment was not excluded: %s", comment.URL)
		}
	}
}

func syntheticUpdatedClient(t *testing.T) *fakeClient {
	t.Helper()
	return &fakeClient{t: t, fixtures: map[string]string{
		requestKey(activityQuery, "", ""):                 "synthetic_search_page1.json",
		requestKey(activityQuery, "", "S1"):               "synthetic_search_page2.json",
		requestKey(reviewsPageQuery, "PR1", "R1"):         "synthetic_reviews_page.json",
		requestKey(commentsPageQuery, "PR1", "C1"):        "synthetic_comments_page.json",
		requestKey(threadsPageQuery, "PR1", "T1"):         "synthetic_threads_page.json",
		requestKey(threadCommentsPageQuery, "TH1", "TC1"): "synthetic_thread_comments_page.json",
	}}
}

func TestFetchUpdatedAcrossAllPaginationLevels(t *testing.T) {
	client := syntheticUpdatedClient(t)

	got, err := fetchUpdated(context.Background(), client, testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authors := make([]string, 0, len(got.Comments))
	for _, comment := range got.Comments {
		authors = append(authors, comment.Author)
	}
	slices.Sort(authors)
	if want := []string{"carol", "dave", "grace", "heidi", "ivan", "judy"}; !slices.Equal(authors, want) {
		t.Errorf("comment authors = %v, want %v", authors, want)
	}

	reviewers := make([]string, 0, len(got.Reviews))
	for _, review := range got.Reviews {
		reviewers = append(reviewers, review.Reviewer)
	}
	slices.Sort(reviewers)
	if want := []string{"bob", "boundary", "erin"}; !slices.Equal(reviewers, want) {
		t.Errorf("reviewers = %v, want %v", reviewers, want)
	}
}

func TestFetchUpdatedTellsIssuesFromPullRequests(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, comment := range got.Comments {
		if want := comment.ParentNumber != 4; comment.OnPullRequest != want {
			t.Errorf("comment on #%d: OnPullRequest = %t, want %t", comment.ParentNumber, comment.OnPullRequest, want)
		}
	}
}

func TestFetchUpdatedStoresReviewTextAndPRAuthor(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	index := slices.IndexFunc(got.Reviews, func(r Review) bool { return r.Reviewer == "bob" })
	if index < 0 {
		t.Fatal("bob's review is missing")
	}
	if review := got.Reviews[index]; review.PRAuthor != "alice" || review.Excerpt != "Looks good" {
		t.Errorf("review = %+v, want PR author alice and text", review)
	}
}

func TestFetchFailedChecksReportsDefaultBranch(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{requestKey(checksQuery, "", ""): "checks_passing.json"}}

	got, err := fetchFailedChecks(context.Background(), client, testRepo, date(t, "2026-10-27T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Branch != "trunk" {
		t.Errorf("branch = %q, want trunk", got.Branch)
	}
}

func TestFetchUpdatedStoresCommentExcerpts(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	index := slices.IndexFunc(got.Comments, func(c Comment) bool { return c.Author == "carol" })
	if index < 0 {
		t.Fatal("carol's comment is missing")
	}
	text := got.Comments[index].Excerpt
	if !strings.HasPrefix(text, "First line second 🔴 line x") || !strings.HasSuffix(text, "…") {
		t.Errorf("excerpt = %q", text)
	}
}

func TestFetchUpdatedSearchesRepoSinceTimestamp(t *testing.T) {
	client := syntheticUpdatedClient(t)

	if _, err := fetchUpdated(context.Background(), client, testRepo, date(t, "2026-10-01T00:00:00-06:00")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "repo:o/r updated:>=2026-10-01T06:00:00Z sort:updated-desc"
	if len(client.searches) == 0 || client.searches[0] != want {
		t.Errorf("searches = %q, want first %q", client.searches, want)
	}
}

func TestFetchUpdatedFiltersPullRequests(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PR 2 is from a bot and PR 5 was updated recently but merged long ago.
	if want := []int{1}; !slices.Equal(prNumbers(got.MergedPRs), want) {
		t.Errorf("merged = %v, want %v", prNumbers(got.MergedPRs), want)
	}
	if want := []int{3}; !slices.Equal(prNumbers(got.OpenedPRs), want) {
		t.Errorf("opened = %v, want %v", prNumbers(got.OpenedPRs), want)
	}
}

func TestFetchUpdatedReportsDeletedAuthorsAsGhost(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.OpenedPRs) != 1 || got.OpenedPRs[0].Author != ghostLogin {
		t.Errorf("opened = %+v, want one PR by %q", got.OpenedPRs, ghostLogin)
	}
}

func TestFetchUpdatedKeepsHumanReviewsOnBotPRs(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	index := slices.IndexFunc(got.Reviews, func(r Review) bool { return r.Reviewer == "erin" })
	if index < 0 || got.Reviews[index].PRNumber != 2 {
		t.Errorf("reviews = %+v, want erin's review on PR 2", got.Reviews)
	}
}

func TestFetchUpdatedWarnsWhenSearchIsTruncated(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "1500") {
		t.Errorf("warnings = %q, want one mentioning 1500", got.Warnings)
	}
}

func TestFetchReviewRequestsAcrossPages(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{
		requestKey(reviewRequestsQuery, "", ""):    "synthetic_review_requests_page1.json",
		requestKey(reviewRequestsQuery, "", "RR1"): "synthetic_review_requests_page2.json",
	}}

	got, err := fetchReviewRequests(context.Background(), client, testRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Viewer != "octocat" {
		t.Errorf("viewer = %q, want octocat", got.Viewer)
	}
	if want := []int{7, 9}; !slices.Equal(prNumbers(got.PullRequests), want) {
		t.Errorf("review requests = %v, want %v", prNumbers(got.PullRequests), want)
	}
	if want := "repo:o/r is:pr is:open review-requested:@me"; client.searches[0] != want {
		t.Errorf("search = %q, want %q", client.searches[0], want)
	}
}

func TestFetchFailedChecksAcrossPages(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{
		requestKey(checksQuery, "", ""):                "synthetic_checks_page1.json",
		requestKey(checksQuery, "", "H1"):              "synthetic_checks_page2.json",
		requestKey(checkContextsPageQuery, "C2", "X1"): "synthetic_check_contexts_page.json",
	}}

	got, err := fetchFailedChecks(context.Background(), client, testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []FailedCheck{
		{Name: "legacy", Conclusion: "ERROR", CommitSHA: "22222222", URL: "https://github.com/o/r/commit/2"},
		{Name: "boot", Conclusion: "STARTUP_FAILURE", CommitSHA: "22222222", URL: "https://github.com/o/r/runs/boot"},
		{Name: "deploy", Conclusion: "FAILURE", CommitSHA: "22222222", URL: "https://ci.example.com/1"},
		{Name: "slow", Conclusion: "TIMED_OUT", CommitSHA: "55555555", URL: "https://github.com/o/r/runs/slow"},
	}
	if !slices.Equal(got.Failed, want) {
		t.Errorf("failed checks = %+v, want %+v", got.Failed, want)
	}
}

func TestFetchFailedChecksOnRepoWithoutCommits(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{requestKey(checksQuery, "", ""): "synthetic_checks_empty_repo.json"}}

	got, err := fetchFailedChecks(context.Background(), client, testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Failed) != 0 {
		t.Errorf("failed checks = %v, want none", got.Failed)
	}
}

func TestFetchWrapsClientErrors(t *testing.T) {
	cause := errors.New("boom")
	client := &fakeClient{t: t, err: cause}

	_, err := FetchWith(context.Background(), client, testRepo, date(t, "2026-10-01T00:00:00Z"))
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want it to wrap %v", err, cause)
	}
}

func TestExcerpt(t *testing.T) {
	long := strings.Repeat("🔴", excerptLength+10)
	tests := []struct {
		name, body, want string
	}{
		{"collapses whitespace", "first\n\n  second\tthird", "first second third"},
		{"keeps short text", "short", "short"},
		{"empty", "", ""},
		{"cuts by rune", long, strings.Repeat("🔴", excerptLength) + "…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := excerpt(tt.body); got != tt.want {
				t.Errorf("excerpt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEveryItemHasURL(t *testing.T) {
	got, err := fetchUpdated(context.Background(), syntheticUpdatedClient(t), testRepo, date(t, "2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var urls []string
	for _, pr := range slices.Concat(got.MergedPRs, got.OpenedPRs) {
		urls = append(urls, pr.URL)
	}
	for _, review := range got.Reviews {
		urls = append(urls, review.URL)
	}
	for _, comment := range got.Comments {
		urls = append(urls, comment.URL)
	}
	if len(urls) == 0 || slices.Contains(urls, "") {
		t.Errorf("urls = %q, want every item to have one", urls)
	}
}
