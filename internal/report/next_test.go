package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/criteria"
	"github.com/David-Alcocer/gh-receipts/internal/pending"
)

func samplePending() pending.Pending {
	return pending.Pending{
		Viewer: "octocat",
		PullRequests: []pending.PullRequest{
			{Number: 44, Title: "Hook", URL: "https://github.com/o/r/pull/44", CIState: "FAILURE", ReviewDecision: "CHANGES_REQUESTED"},
			{Number: 46, Title: "Typo", URL: "https://github.com/o/r/pull/46", Draft: true},
		},
		ReviewRequests: []activity.PullRequest{{Number: 45, Title: "Redis", Author: "luis", URL: "https://github.com/o/r/pull/45"}},
		Issues: []pending.Issue{
			{Number: 39, Title: "Signup", URL: "https://github.com/o/r/issues/39", Criteria: criteria.Parse("## Acceptance criteria\n- [x] A\n- [ ] B\n")},
			{Number: 41, Title: "Limits", URL: "https://github.com/o/r/issues/41", Criteria: criteria.Parse("nothing")},
			{Number: 42, Title: "Empty", URL: "https://github.com/o/r/issues/42", Criteria: criteria.Parse("## Acceptance criteria\n")},
		},
	}
}

func renderNext(t *testing.T, p pending.Pending) string {
	t.Helper()
	var out bytes.Buffer
	if err := NextText(&out, testRepo, p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out.String()
}

func TestNextTextShowsSectionsInOrder(t *testing.T) {
	want := `YOUR OPEN PRS
  #44 Hook  CI failed, changes requested      https://github.com/o/r/pull/44
  #46 Typo  no CI, no review decision, draft  https://github.com/o/r/pull/46

WAITING FOR YOUR REVIEW
  #45 Redis (luis)  https://github.com/o/r/pull/45

ISSUES ASSIGNED TO YOU
  #39 Signup  1/2                https://github.com/o/r/issues/39
  #41 Limits  no criteria found  https://github.com/o/r/issues/41
  #42 Empty  section is empty    https://github.com/o/r/issues/42
`
	if got := renderNext(t, samplePending()); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestNextTextEveryLineEndsWithAURL(t *testing.T) {
	titles := map[string]bool{"YOUR OPEN PRS": true, "WAITING FOR YOUR REVIEW": true, "ISSUES ASSIGNED TO YOU": true, "": true}
	for _, line := range strings.Split(renderNext(t, samplePending()), "\n") {
		fields := strings.Fields(line)
		if titles[line] {
			continue
		}
		if !strings.HasPrefix(fields[len(fields)-1], "https://github.com/") {
			t.Errorf("line without a source: %q", line)
		}
	}
}

func TestNextTextDescribesCheckAndReviewStates(t *testing.T) {
	tests := []struct {
		state, decision, want string
	}{
		{"SUCCESS", "APPROVED", "CI passed, approved"},
		{"FAILURE", "REVIEW_REQUIRED", "CI failed, review required"},
		{"ERROR", "CHANGES_REQUESTED", "CI failed, changes requested"},
		{"PENDING", "", "CI running, no review decision"},
		{"EXPECTED", "", "CI running, no review decision"},
		{"", "", "no CI, no review decision"},
		{"NEW_STATE", "NEW_DECISION", "CI NEW_STATE, NEW_DECISION"},
	}

	for _, tt := range tests {
		pr := pending.PullRequest{CIState: tt.state, ReviewDecision: tt.decision}
		if got := pullRequestStatus(pr); got != tt.want {
			t.Errorf("status(%q, %q) = %q, want %q", tt.state, tt.decision, got, tt.want)
		}
	}
}

func TestNextTextHidesEmptySections(t *testing.T) {
	got := renderNext(t, pending.Pending{ReviewRequests: []activity.PullRequest{{Number: 1, Title: "T", Author: "a", URL: "u"}}})

	if strings.Contains(got, "YOUR OPEN PRS") || strings.Contains(got, "ISSUES ASSIGNED") || !strings.HasPrefix(got, "WAITING FOR YOUR REVIEW\n") {
		t.Errorf("text = %q", got)
	}
}

func TestNextTextNothingPendingLinksTheRepository(t *testing.T) {
	want := "Nothing pending  https://github.com/o/r\n"

	if got := renderNext(t, pending.Pending{}); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestNextTextLimitsLongSections(t *testing.T) {
	var p pending.Pending
	for i := 1; i <= sectionLimit+2; i++ {
		p.Issues = append(p.Issues, pending.Issue{Number: i, Title: "T", URL: fmt.Sprintf("https://github.com/o/r/issues/%d", i)})
	}

	got := renderNext(t, p)

	if !strings.Contains(got, "… and 2 more") || !strings.Contains(got, "https://github.com/o/r/issues?q=is%3Aissue+is%3Aopen+assignee%3A%40me") {
		t.Errorf("text = %q", got)
	}
}

func TestNextTextLinksEachSectionToItsFullList(t *testing.T) {
	var p pending.Pending
	for i := 1; i <= sectionLimit+1; i++ {
		p.PullRequests = append(p.PullRequests, pending.PullRequest{Number: i, Title: "T", URL: "u"})
		p.ReviewRequests = append(p.ReviewRequests, activity.PullRequest{Number: i, Title: "T", Author: "a", URL: "u"})
	}

	got := renderNext(t, p)

	for _, want := range []string{"pulls?q=is%3Apr+is%3Aopen+author%3A%40me", "pulls?q=is%3Apr+is%3Aopen+review-requested%3A%40me"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestNextTextRemovesControlCharacters(t *testing.T) {
	p := pending.Pending{
		PullRequests:   []pending.PullRequest{{Number: 1, Title: "A\x1b[2J", URL: "u"}},
		ReviewRequests: []activity.PullRequest{{Number: 2, Title: "B\x1b[2J", Author: "a", URL: "u"}},
		Issues:         []pending.Issue{{Number: 3, Title: "C\x1b[2J", URL: "u"}},
	}

	if got := renderNext(t, p); strings.Contains(got, "\x1b") {
		t.Errorf("text = %q", got)
	}
}

func TestNextJSONIsComplete(t *testing.T) {
	p := samplePending()
	p.Warnings = []string{"careful"}
	for i := 0; i <= sectionLimit; i++ {
		p.Issues = append(p.Issues, pending.Issue{Number: 100 + i})
	}
	var out bytes.Buffer

	if err := NextJSON(&out, testRepo, p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document struct {
		SchemaVersion int
		Repository    string
		Viewer        string
		PullRequests  []struct {
			CIState        string `json:"ciState"`
			ReviewDecision string `json:"reviewDecision"`
			Draft          bool   `json:"draft"`
		}
		ReviewRequests []activity.PullRequest
		Issues         []struct {
			Criteria criteria.Result
		}
		Warnings []string
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || document.Repository != "o/r" || document.Viewer != "octocat" || len(document.Warnings) != 1 {
		t.Errorf("document = %+v", document)
	}
	if document.PullRequests[0].CIState != "FAILURE" || document.PullRequests[0].ReviewDecision != "CHANGES_REQUESTED" || !document.PullRequests[1].Draft {
		t.Errorf("pull requests = %+v", document.PullRequests)
	}
	if len(document.Issues) != 3+sectionLimit+1 {
		t.Errorf("issues = %d", len(document.Issues))
	}
	if items := document.Issues[0].Criteria.Items; len(items) != 2 || items[1].Text != "B" || items[1].Checked {
		t.Errorf("criteria = %+v", items)
	}
}

func TestNextJSONUsesEmptyLists(t *testing.T) {
	var out bytes.Buffer

	if err := NextJSON(&out, testRepo, pending.Pending{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out.String(), "null") {
		t.Errorf("JSON = %s", out.String())
	}
}
