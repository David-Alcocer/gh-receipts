package report

import (
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/project"
)

const (
	// sectionLimit keeps the text report short enough to read. The JSON output
	// has no limit.
	sectionLimit = 10
	// lineExcerptLength is the most characters of a comment shown on one line.
	lineExcerptLength = 60
	shortSHALength    = 7
	timeLayout        = "Mon 2 Jan, 15:04"

	stateApproved         = "APPROVED"
	stateChangesRequested = "CHANGES_REQUESTED"
)

type line struct {
	text string
	url  string
}

type section struct {
	title string
	lines []line
	// moreURL opens the full list when the section goes over sectionLimit.
	moreURL string
}

// Text writes the sections that need attention, with actionable ones first.
func Text(w io.Writer, a activity.Activity, info Info) error {
	since := info.Since.Format(timeLayout)
	sections := visibleSections(a, info)
	if len(sections) == 0 {
		_, err := fmt.Fprintf(w, "Nothing new since %s\n", since)
		return err
	}

	if _, err := fmt.Fprintf(w, "Since %s (%s)\n", since, info.SinceSource); err != nil {
		return err
	}
	for _, s := range sections {
		if err := writeSection(w, s); err != nil {
			return err
		}
	}
	return nil
}

func visibleSections(a activity.Activity, info Info) []section {
	base := repositoryURL(info.Repo)
	all := []section{
		{
			title:   "WAITING FOR YOUR REVIEW",
			lines:   pullRequestLines(a.ReviewRequests),
			moreURL: base + "/pulls?q=" + url.QueryEscape("is:pr is:open review-requested:@me"),
		},
		{
			title:   "NEW COMMENTS ON YOUR PRS",
			lines:   feedbackLines(a),
			moreURL: base + "/pulls?q=" + url.QueryEscape("is:pr author:@me sort:updated-desc"),
		},
		{
			title:   "FAILED CI ON " + strings.ToUpper(a.DefaultBranch),
			lines:   failedCheckLines(a.FailedChecks),
			moreURL: base + "/actions?query=" + url.QueryEscape("is:failure branch:"+a.DefaultBranch),
		},
		{
			title:   "MERGED",
			lines:   pullRequestLines(a.MergedPRs),
			moreURL: base + "/pulls?q=" + url.QueryEscape("is:pr is:merged merged:>="+info.Since.UTC().Format(time.DateOnly)),
		},
	}
	if info.Project != nil {
		all = append(all, section{title: "PROJECT", lines: projectLines(info.Project.Items), moreURL: info.Project.URL})
	}
	return slices.DeleteFunc(all, func(s section) bool { return len(s.lines) == 0 })
}

func writeSection(w io.Writer, s section) error {
	if _, err := fmt.Fprintf(w, "\n%s\n", s.title); err != nil {
		return err
	}

	shown := s.lines[:min(len(s.lines), sectionLimit)]
	columns := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, l := range shown {
		fmt.Fprintf(columns, "  %s\t%s\n", l.text, l.url)
	}
	if hidden := len(s.lines) - len(shown); hidden > 0 {
		fmt.Fprintf(columns, "  … and %d more\t%s\n", hidden, s.moreURL)
	}
	return columns.Flush()
}

func pullRequestLines(prs []activity.PullRequest) []line {
	lines := make([]line, 0, len(prs))
	for _, pr := range prs {
		lines = append(lines, line{text: fmt.Sprintf("#%d %s (%s)", pr.Number, clean(pr.Title), pr.Author), url: pr.URL})
	}
	return lines
}

// feedbackLines returns what others said on the viewer's pull requests, newest
// first.
func feedbackLines(a activity.Activity) []line {
	type dated struct {
		line
		at time.Time
	}
	var items []dated

	for _, c := range a.Comments {
		if c.OnPullRequest && c.ParentAuthor == a.Viewer && c.Author != a.Viewer {
			text := fmt.Sprintf("#%d %q (%s)", c.ParentNumber, shorten(c.Excerpt), c.Author)
			items = append(items, dated{line{text, c.URL}, c.CreatedAt})
		}
	}
	for _, r := range a.Reviews {
		if r.PRAuthor != a.Viewer || r.Reviewer == a.Viewer {
			continue
		}
		if text, ok := reviewText(r); ok {
			items = append(items, dated{line{text, r.URL}, r.SubmittedAt})
		}
	}

	slices.SortStableFunc(items, func(x, y dated) int { return y.at.Compare(x.at) })
	lines := make([]line, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.line)
	}
	return lines
}

// reviewText describes a review. A review with no verdict and no text only
// groups inline comments, which are listed on their own, so it has none.
func reviewText(r activity.Review) (string, bool) {
	quote := ""
	if r.Excerpt != "" {
		quote = fmt.Sprintf("%q", shorten(r.Excerpt))
	}

	switch {
	case r.State == stateChangesRequested && quote != "":
		return fmt.Sprintf("#%d Changes requested: %s (%s)", r.PRNumber, quote, r.Reviewer), true
	case r.State == stateChangesRequested:
		return fmt.Sprintf("#%d Changes requested (%s)", r.PRNumber, r.Reviewer), true
	case r.State == stateApproved && quote != "":
		return fmt.Sprintf("#%d Approved: %s (%s)", r.PRNumber, quote, r.Reviewer), true
	case r.State == stateApproved:
		return fmt.Sprintf("#%d Approved (%s)", r.PRNumber, r.Reviewer), true
	case quote != "":
		return fmt.Sprintf("#%d %s (%s)", r.PRNumber, quote, r.Reviewer), true
	}
	return "", false
}

func failedCheckLines(checks []activity.FailedCheck) []line {
	lines := make([]line, 0, len(checks))
	for _, check := range checks {
		sha := check.CommitSHA[:min(len(check.CommitSHA), shortSHALength)]
		lines = append(lines, line{text: fmt.Sprintf("%s @ %s", clean(check.Name), sha), url: check.URL})
	}
	return lines
}

// clean makes text written by others safe to print: control characters could
// otherwise move the cursor or change the terminal.
func clean(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, text)
}

func projectLines(items []project.Item) []line {
	lines := make([]line, 0, len(items))
	for _, item := range items {
		text := fmt.Sprintf("%s  %s", clean(item.Title), clean(item.Status))
		if item.Number != 0 {
			text = fmt.Sprintf("#%d %s", item.Number, text)
		}
		lines = append(lines, line{text: text, url: item.URL})
	}
	return lines
}

func shorten(text string) string {
	runes := []rune(text)
	if len(runes) <= lineExcerptLength {
		return text
	}
	return string(runes[:lineExcerptLength]) + "…"
}
