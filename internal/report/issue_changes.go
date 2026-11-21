package report

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
)

// closeReasons describes the state reasons GitHub documents. Others are shown
// as they came.
var closeReasons = map[string]string{
	"COMPLETED":   "closed as completed",
	"NOT_PLANNED": "closed as not planned",
	"DUPLICATE":   "closed as duplicate",
}

func issueChangesSection(repo repository.Repository, a activity.Activity) section {
	return section{
		title:   "CHANGES ON YOUR ISSUES",
		lines:   issueChangeLines(a),
		moreURL: repositoryURL(repo) + "/issues?q=" + url.QueryEscape("is:issue assignee:@me sort:updated-desc"),
	}
}

// issueChangeLines lists comments and changes by other people on the viewer's
// issues, newest first. Changes are where a team changes direction: a comment,
// edited criteria, an issue closed as not planned or given to someone else.
func issueChangeLines(a activity.Activity) []line {
	yours := viewerIssues(a)
	type dated struct {
		line
		at time.Time
	}
	var items []dated

	for _, c := range a.Comments {
		// Issues and pull requests share numbers, so a comment on a pull request
		// never matches one of the viewer's issues.
		if yours[c.ParentNumber] && c.Author != a.Viewer {
			text := fmt.Sprintf("#%d %q (%s)", c.ParentNumber, shorten(clean(c.Excerpt)), c.Author)
			items = append(items, dated{line{text, c.URL}, c.CreatedAt})
		}
	}
	for _, e := range a.IssueEvents {
		if yours[e.IssueNumber] && e.Actor != a.Viewer {
			text := fmt.Sprintf("#%d %s (%s)", e.IssueNumber, describeIssueEvent(e, a.Viewer), e.Actor)
			items = append(items, dated{line{text, e.URL}, e.CreatedAt})
		}
	}

	slices.SortStableFunc(items, func(x, y dated) int { return y.at.Compare(x.at) })
	lines := make([]line, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.line)
	}
	return lines
}

// viewerIssues are the issues assigned to the viewer, and the ones that stopped
// being theirs in the window, so the report can say so.
func viewerIssues(a activity.Activity) map[int]bool {
	yours := map[int]bool{}
	for _, issue := range a.Issues {
		if slices.Contains(issue.Assignees, a.Viewer) {
			yours[issue.Number] = true
		}
	}
	for _, e := range a.IssueEvents {
		if e.Kind == activity.EventUnassigned && e.Detail == a.Viewer {
			yours[e.IssueNumber] = true
		}
	}
	return yours
}

func describeIssueEvent(e activity.IssueEvent, viewer string) string {
	detail := shorten(clean(e.Detail))
	switch e.Kind {
	case activity.EventClosed:
		if e.Detail == "" {
			return "closed"
		}
		if reason, known := closeReasons[e.Detail]; known {
			return reason
		}
		return "closed as " + detail
	case activity.EventAssigned:
		return "assigned to " + you(detail, viewer)
	case activity.EventUnassigned:
		return "unassigned from " + you(detail, viewer)
	case activity.EventRenamed:
		return fmt.Sprintf("renamed to %q", detail)
	case activity.EventLabeled:
		return fmt.Sprintf("labeled %q", detail)
	case activity.EventUnlabeled:
		return fmt.Sprintf("label %q removed", detail)
	case activity.EventMilestoned:
		return fmt.Sprintf("milestone %q set", detail)
	case activity.EventDemilestoned:
		return fmt.Sprintf("milestone %q removed", detail)
	case activity.EventEdited:
		return "description edited"
	}
	return strings.TrimSpace(clean(e.Kind) + " " + detail)
}

func you(login, viewer string) string {
	if login == viewer {
		return "you"
	}
	return login
}
