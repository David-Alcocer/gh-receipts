package report

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
)

var update = flag.Bool("update", false, "rewrite the golden files")

var (
	testRepo  = repository.Repository{Host: "github.com", Owner: "o", Name: "r"}
	testSince = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	testInfo  = Info{Repo: testRepo, Since: testSince, SinceSource: "last session"}
)

// fixtureClient answers queries from the activity package's fixtures, keyed by
// operation name, node ID and cursor.
type fixtureClient struct{ t *testing.T }

var fixtures = map[string]string{
	"Activity||":                 "synthetic_search_page1.json",
	"Activity||S1":               "synthetic_search_page2.json",
	"ReviewsPage|PR1|R1":         "synthetic_reviews_page.json",
	"CommentsPage|PR1|C1":        "synthetic_comments_page.json",
	"ThreadsPage|PR1|T1":         "synthetic_threads_page.json",
	"ThreadCommentsPage|TH1|TC1": "synthetic_thread_comments_page.json",
	"ReviewRequests||":           "synthetic_review_requests_page1.json",
	"ReviewRequests||RR1":        "synthetic_review_requests_page2.json",
	"Checks||":                   "synthetic_checks_page1.json",
	"Checks||H1":                 "synthetic_checks_page2.json",
	"CheckContextsPage|C2|X1":    "synthetic_check_contexts_page.json",
}

func (c fixtureClient) DoWithContext(_ context.Context, query string, variables map[string]interface{}, response interface{}) error {
	c.t.Helper()
	operation := strings.SplitN(strings.Fields(query)[1], "(", 2)[0]
	id, _ := variables["id"].(string)
	cursor, _ := variables["cursor"].(string)
	key := fmt.Sprintf("%s|%s|%s", operation, id, cursor)

	name, ok := fixtures[key]
	if !ok {
		c.t.Fatalf("no fixture for %s", key)
	}
	data, err := os.ReadFile(filepath.Join("..", "activity", "testdata", name))
	if err != nil {
		c.t.Fatal(err)
	}
	var envelope struct{ Data json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		c.t.Fatal(err)
	}
	return json.Unmarshal(envelope.Data, response)
}

// fixtureActivity returns the activity from the fixtures, seen by alice, who
// authored PR 1.
func fixtureActivity(t *testing.T) activity.Activity {
	t.Helper()
	got, err := activity.FetchWith(context.Background(), fixtureClient{t}, testRepo, testSince)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got.Viewer = "alice"
	return got
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

func renderText(t *testing.T, a activity.Activity) string {
	t.Helper()
	var out bytes.Buffer
	if err := Text(&out, a, testInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out.String()
}

func TestTextMatchesGolden(t *testing.T) {
	assertGolden(t, "today.golden", renderText(t, fixtureActivity(t)))
}

func TestJSONMatchesGolden(t *testing.T) {
	var out bytes.Buffer
	if err := JSON(&out, fixtureActivity(t), testInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertGolden(t, "today.json.golden", out.String())
}

func TestTextPutsActionableSectionsFirst(t *testing.T) {
	got := renderText(t, fixtureActivity(t))

	var positions []int
	for _, title := range []string{"WAITING FOR YOUR REVIEW", "NEW COMMENTS ON YOUR PRS", "FAILED CI ON MAIN", "MERGED"} {
		position := strings.Index(got, title)
		if position < 0 {
			t.Fatalf("section %q is missing from:\n%s", title, got)
		}
		positions = append(positions, position)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] < positions[i-1] {
			t.Errorf("sections are out of order:\n%s", got)
		}
	}
}

func TestTextEveryContentLineEndsWithURL(t *testing.T) {
	endsWithURL := regexp.MustCompile(`https://\S+$`)

	for _, line := range strings.Split(renderText(t, fixtureActivity(t)), "\n") {
		if strings.HasPrefix(line, "  ") && !endsWithURL.MatchString(line) {
			t.Errorf("line has no source: %q", line)
		}
	}
}

func TestTextHidesEmptySections(t *testing.T) {
	a := activity.Activity{Viewer: "alice", MergedPRs: []activity.PullRequest{{Number: 1, Title: "One", Author: "bob", URL: "https://github.com/o/r/pull/1"}}}

	got := renderText(t, a)

	if !strings.Contains(got, "MERGED") {
		t.Errorf("merged section is missing:\n%s", got)
	}
	for _, title := range []string{"WAITING", "NEW COMMENTS", "FAILED CI"} {
		if strings.Contains(got, title) {
			t.Errorf("empty section %q is shown:\n%s", title, got)
		}
	}
}

func TestTextWithNothingNewPrintsOneLine(t *testing.T) {
	a := activity.Activity{Viewer: "alice", Comments: []activity.Comment{{Author: "bob", ParentAuthor: "carol", OnPullRequest: true, URL: "https://github.com/o/r/pull/1#c"}}}

	if got, want := renderText(t, a), "Nothing new since Thu 1 Oct, 00:00\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestTextLimitsSectionsAndLinksToTheRest(t *testing.T) {
	prs := make([]activity.PullRequest, 13)
	for i := range prs {
		prs[i] = activity.PullRequest{Number: i + 1, Title: "PR", Author: "bob", URL: fmt.Sprintf("https://github.com/o/r/pull/%d", i+1)}
	}

	got := renderText(t, activity.Activity{Viewer: "alice", MergedPRs: prs})

	if count := strings.Count(got, "/pull/"); count != 10 {
		t.Errorf("printed %d PRs, want 10:\n%s", count, got)
	}
	if !strings.Contains(got, "… and 3 more") {
		t.Errorf("missing the line for the rest:\n%s", got)
	}
	if !strings.Contains(got, "https://github.com/o/r/pulls?q=") {
		t.Errorf("the line for the rest has no link:\n%s", got)
	}
}

func TestJSONIsNeverLimited(t *testing.T) {
	prs := make([]activity.PullRequest, sectionLimit+3)
	var out bytes.Buffer
	if err := JSON(&out, activity.Activity{MergedPRs: prs}, testInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got struct{ MergedPRs []activity.PullRequest }
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.MergedPRs) != len(prs) {
		t.Errorf("merged = %d, want %d", len(got.MergedPRs), len(prs))
	}
}

func TestJSONUsesEmptyListsInsteadOfNull(t *testing.T) {
	var out bytes.Buffer
	if err := JSON(&out, activity.Activity{}, testInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "null") {
		t.Errorf("output contains null:\n%s", out.String())
	}
}

func TestReviewsOnYourPRs(t *testing.T) {
	tests := []struct {
		name   string
		review activity.Review
		want   string
	}{
		{"changes requested", activity.Review{PRNumber: 1, PRAuthor: "alice", Reviewer: "bob", State: "CHANGES_REQUESTED", Excerpt: "Split this", URL: "https://x/1"}, `#1 Changes requested: "Split this" (bob)`},
		{"approved", activity.Review{PRNumber: 1, PRAuthor: "alice", Reviewer: "bob", State: "APPROVED", URL: "https://x/2"}, "#1 Approved (bob)"},
		{"comment with text", activity.Review{PRNumber: 1, PRAuthor: "alice", Reviewer: "bob", State: "COMMENTED", Excerpt: "Nice", URL: "https://x/3"}, `#1 "Nice" (bob)`},
		{"empty container of inline comments", activity.Review{PRNumber: 1, PRAuthor: "alice", Reviewer: "bob", State: "COMMENTED", URL: "https://x/4"}, ""},
		{"someone else's PR", activity.Review{PRNumber: 1, PRAuthor: "carol", Reviewer: "bob", State: "APPROVED", URL: "https://x/5"}, ""},
		{"your own review", activity.Review{PRNumber: 1, PRAuthor: "alice", Reviewer: "alice", State: "COMMENTED", Excerpt: "Note", URL: "https://x/6"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderText(t, activity.Activity{Viewer: "alice", Reviews: []activity.Review{tt.review}})
			if tt.want == "" {
				if strings.Contains(got, "NEW COMMENTS") {
					t.Errorf("review should be hidden:\n%s", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("output misses %q:\n%s", tt.want, got)
			}
		})
	}
}

func TestCommentsOnYourPRs(t *testing.T) {
	tests := []struct {
		name    string
		comment activity.Comment
		shown   bool
	}{
		{"on your PR", activity.Comment{ParentNumber: 1, ParentAuthor: "alice", OnPullRequest: true, Author: "bob", Excerpt: "Hi", URL: "https://x/1"}, true},
		{"your own comment", activity.Comment{ParentNumber: 1, ParentAuthor: "alice", OnPullRequest: true, Author: "alice", Excerpt: "Hi", URL: "https://x/2"}, false},
		{"on your issue", activity.Comment{ParentNumber: 1, ParentAuthor: "alice", OnPullRequest: false, Author: "bob", Excerpt: "Hi", URL: "https://x/3"}, false},
		{"on someone else's PR", activity.Comment{ParentNumber: 1, ParentAuthor: "carol", OnPullRequest: true, Author: "bob", Excerpt: "Hi", URL: "https://x/4"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderText(t, activity.Activity{Viewer: "alice", Comments: []activity.Comment{tt.comment}})
			if shown := strings.Contains(got, "NEW COMMENTS"); shown != tt.shown {
				t.Errorf("shown = %t, want %t:\n%s", shown, tt.shown, got)
			}
		})
	}
}

func TestTextRemovesControlCharactersFromUntrustedText(t *testing.T) {
	a := activity.Activity{
		MergedPRs:     []activity.PullRequest{{Number: 1, Title: "evil\x1b[2Jtitle\twith tab", Author: "bob", URL: "https://x/1"}},
		DefaultBranch: "main",
		FailedChecks:  []activity.FailedCheck{{Name: "bad\x1b]0;name", CommitSHA: "abc", URL: "https://x/2"}},
	}

	got := renderText(t, a)

	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("output keeps an escape character: %q", got)
	}
	if !strings.Contains(got, "evil[2Jtitle with tab") {
		t.Errorf("unexpected title in %q", got)
	}
}

func TestFailedCISectionUsesTheRealBranch(t *testing.T) {
	a := activity.Activity{DefaultBranch: "trunk", FailedChecks: []activity.FailedCheck{{Name: "lint", CommitSHA: "3f2a1c9aaaa", URL: "https://x/1"}}}

	got := renderText(t, a)

	if !strings.Contains(got, "FAILED CI ON TRUNK") || !strings.Contains(got, "lint @ 3f2a1c9") {
		t.Errorf("unexpected output:\n%s", got)
	}
}
