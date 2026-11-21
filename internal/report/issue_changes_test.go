package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/pending"
)

func issueURL(number int) string { return fmt.Sprintf("https://github.com/o/r/issues/%d", number) }

func issueEvent(number int, kind, actor, detail string, hour int) activity.IssueEvent {
	return activity.IssueEvent{IssueNumber: number, Kind: kind, Actor: actor, Detail: detail, CreatedAt: testSince.Add(time.Duration(hour) * time.Hour), URL: issueURL(number)}
}

func issueComment(number int, author, text string, hour int) activity.Comment {
	return activity.Comment{Author: author, Excerpt: text, CreatedAt: testSince.Add(time.Duration(hour) * time.Hour), URL: issueURL(number) + "#issuecomment-1", ParentNumber: number}
}

// issueActivity has issue 39 assigned to the viewer, 50 assigned to someone
// else, and 44 that was the viewer's until it was unassigned.
func issueActivity() activity.Activity {
	return activity.Activity{
		Viewer: "me",
		Issues: []activity.Issue{
			{Number: 39, Title: "Signup", URL: issueURL(39), State: "OPEN", Assignees: []string{"me"}},
			{Number: 50, Title: "Other", URL: issueURL(50), State: "OPEN", Assignees: []string{"bob"}},
			{Number: 44, Title: "Gone", URL: issueURL(44), State: "OPEN", Assignees: []string{"bob"}},
		},
		Comments: []activity.Comment{
			issueComment(39, "ana", "Let's drop email login", 9),
			issueComment(39, "me", "ok", 10),
			issueComment(50, "ana", "not yours", 11),
		},
		IssueEvents: []activity.IssueEvent{
			issueEvent(39, activity.EventEdited, "ana", "", 8),
			issueEvent(39, activity.EventLabeled, "me", "mine", 12),
			issueEvent(50, activity.EventClosed, "luis", "NOT_PLANNED", 13),
			issueEvent(44, activity.EventUnassigned, "luis", "me", 7),
		},
	}
}

func TestIssueChangesShowOnlyTheViewersIssuesByOthers(t *testing.T) {
	want := `CHANGES ON YOUR ISSUES
  #39 "Let's drop email login" (ana)  https://github.com/o/r/issues/39#issuecomment-1
  #39 description edited (ana)        https://github.com/o/r/issues/39
  #44 unassigned from you (luis)      https://github.com/o/r/issues/44
`

	got := renderText(t, issueActivity())

	if !strings.Contains(got, want) {
		t.Errorf("text =\n%s\nwant to contain\n%s", got, want)
	}
}

func TestIssueChangesDescribeEveryKind(t *testing.T) {
	tests := []struct {
		kind, detail, want string
	}{
		{activity.EventClosed, "NOT_PLANNED", "#39 closed as not planned (luis)"},
		{activity.EventClosed, "COMPLETED", "#39 closed as completed (luis)"},
		{activity.EventClosed, "DUPLICATE", "#39 closed as duplicate (luis)"},
		{activity.EventClosed, "", "#39 closed (luis)"},
		{activity.EventClosed, "SOMETHING_NEW", "#39 closed as SOMETHING_NEW (luis)"},
		{activity.EventReopened, "", "#39 reopened (luis)"},
		{activity.EventAssigned, "me", "#39 assigned to you (luis)"},
		{activity.EventAssigned, "bob", "#39 assigned to bob (luis)"},
		{activity.EventUnassigned, "bob", "#39 unassigned from bob (luis)"},
		{activity.EventRenamed, "New\x1b[2J title", `#39 renamed to "New[2J title" (luis)`},
		{activity.EventLabeled, "blocked", `#39 labeled "blocked" (luis)`},
		{activity.EventUnlabeled, "ready", `#39 label "ready" removed (luis)`},
		{activity.EventMilestoned, "v1", `#39 milestone "v1" set (luis)`},
		{activity.EventDemilestoned, "v0", `#39 milestone "v0" removed (luis)`},
		{activity.EventEdited, "", "#39 description edited (luis)"},
	}

	for _, tt := range tests {
		t.Run(tt.kind+" "+tt.detail, func(t *testing.T) {
			a := activity.Activity{
				Viewer:      "me",
				Issues:      []activity.Issue{{Number: 39, URL: issueURL(39), Assignees: []string{"me"}}},
				IssueEvents: []activity.IssueEvent{issueEvent(39, tt.kind, "luis", tt.detail, 1)},
			}

			if got := renderText(t, a); !strings.Contains(got, "  "+tt.want+"  ") {
				t.Errorf("text =\n%s\nwant line %q", got, tt.want)
			}
		})
	}
}

func TestIssueChangesComeRightAfterReviewRequestsInToday(t *testing.T) {
	a := issueActivity()
	a.ReviewRequests = []activity.PullRequest{{Number: 7, Title: "PR", Author: "a", URL: "https://github.com/o/r/pull/7"}}
	a.MergedPRs = []activity.PullRequest{{Number: 8, Title: "PR", Author: "a", URL: "https://github.com/o/r/pull/8"}}

	got := renderText(t, a)

	review, changes, merged := strings.Index(got, "WAITING FOR YOUR REVIEW"), strings.Index(got, "CHANGES ON YOUR ISSUES"), strings.Index(got, "MERGED")
	if !(review < changes && changes < merged) {
		t.Errorf("text =\n%s", got)
	}
}

func TestIssueChangesComeRightAfterAssignedIssuesInContext(t *testing.T) {
	a := issueActivity()
	a.MergedPRs = []activity.PullRequest{{Number: 8, Title: "PR", Author: "a", URL: "https://github.com/o/r/pull/8"}}
	p := pending.Pending{Issues: []pending.Issue{{Number: 39, Title: "Signup", URL: issueURL(39)}}}

	got := renderContext(t, Context{Info: testInfo, Activity: &a, Pending: &p})

	assigned, changes, merged := strings.Index(got, "ISSUES ASSIGNED TO YOU"), strings.Index(got, "CHANGES ON YOUR ISSUES"), strings.Index(got, "MERGED")
	if !(assigned < changes && changes < merged) {
		t.Errorf("text =\n%s", got)
	}
}

func TestIssueChangesAreLimitedAndLinkToTheViewersIssues(t *testing.T) {
	a := activity.Activity{Viewer: "me", Issues: []activity.Issue{{Number: 39, URL: issueURL(39), Assignees: []string{"me"}}}}
	for i := range sectionLimit + 1 {
		a.IssueEvents = append(a.IssueEvents, issueEvent(39, activity.EventLabeled, "ana", fmt.Sprint(i), i))
	}

	got := renderText(t, a)

	if !strings.Contains(got, "… and 1 more") || !strings.Contains(got, "issues?q=is%3Aissue+assignee%3A%40me+sort%3Aupdated-desc") {
		t.Errorf("text =\n%s", got)
	}
}

func TestJSONIncludesIssuesAndTheirChanges(t *testing.T) {
	var out bytes.Buffer
	if err := JSON(&out, issueActivity(), testInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document struct {
		Issues      []activity.Issue      `json:"issues"`
		IssueEvents []activity.IssueEvent `json:"issueEvents"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(document.Issues) != 3 || len(document.IssueEvents) != 4 {
		t.Errorf("issues = %d, events = %d, want every one, not only the viewer's", len(document.Issues), len(document.IssueEvents))
	}
}

func TestJSONUsesEmptyListsForIssues(t *testing.T) {
	var out bytes.Buffer
	if err := JSON(&out, activity.Activity{}, testInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{`"issues": []`, `"issueEvents": []`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("JSON missing %s:\n%s", want, out.String())
		}
	}
}
