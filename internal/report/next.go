package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/pending"
)

// nextSchemaVersion changes when the JSON output changes in an incompatible way.
// docs/next-json.md documents the schema.
const nextSchemaVersion = 1

type nextDocument struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Repository     string                 `json:"repository"`
	Viewer         string                 `json:"viewer"`
	PullRequests   []pending.PullRequest  `json:"pullRequests"`
	ReviewRequests []activity.PullRequest `json:"reviewRequests"`
	Issues         []pending.Issue        `json:"issues"`
	Warnings       []string               `json:"warnings"`
}

// NextJSON writes all the pending work. Unlike NextText, it never limits
// sections.
func NextJSON(w io.Writer, repo repository.Repository, p pending.Pending) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(nextDocument{
		SchemaVersion:  nextSchemaVersion,
		Repository:     repo.Owner + "/" + repo.Name,
		Viewer:         p.Viewer,
		PullRequests:   nonNil(p.PullRequests),
		ReviewRequests: nonNil(p.ReviewRequests),
		Issues:         nonNil(p.Issues),
		Warnings:       nonNil(p.Warnings),
	})
	if err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

// NextText writes the sections with pending work, or one line if there is none.
func NextText(w io.Writer, repo repository.Repository, p pending.Pending) error {
	sections := nextSections(repo, p)
	if len(sections) == 0 {
		_, err := fmt.Fprintf(w, "Nothing pending  %s\n", repositoryURL(repo))
		return err
	}

	var out bytes.Buffer
	for _, s := range sections {
		if err := writeSection(&out, s); err != nil {
			return err
		}
	}
	// writeSection separates sections with a blank line, and nothing comes before
	// the first one here.
	_, err := io.WriteString(w, strings.TrimPrefix(out.String(), "\n"))
	return err
}

func nextSections(repo repository.Repository, p pending.Pending) []section {
	base := repositoryURL(repo)
	all := []section{
		{
			title:   "YOUR OPEN PRS",
			lines:   openPullRequestLines(p.PullRequests),
			moreURL: base + "/pulls?q=" + url.QueryEscape("is:pr is:open author:@me"),
		},
		{
			title:   "WAITING FOR YOUR REVIEW",
			lines:   pullRequestLines(p.ReviewRequests),
			moreURL: base + "/pulls?q=" + url.QueryEscape("is:pr is:open review-requested:@me"),
		},
		{
			title:   "ISSUES ASSIGNED TO YOU",
			lines:   assignedIssueLines(p.Issues),
			moreURL: base + "/issues?q=" + url.QueryEscape("is:issue is:open assignee:@me"),
		},
	}
	return slices.DeleteFunc(all, func(s section) bool { return len(s.lines) == 0 })
}

func repositoryURL(repo repository.Repository) string {
	return fmt.Sprintf("https://%s/%s/%s", repo.Host, repo.Owner, repo.Name)
}

func openPullRequestLines(prs []pending.PullRequest) []line {
	lines := make([]line, 0, len(prs))
	for _, pr := range prs {
		text := fmt.Sprintf("#%d %s  %s", pr.Number, clean(pr.Title), pullRequestStatus(pr))
		lines = append(lines, line{text: text, url: pr.URL})
	}
	return lines
}

func assignedIssueLines(issues []pending.Issue) []line {
	lines := make([]line, 0, len(issues))
	for _, issue := range issues {
		text := fmt.Sprintf("#%d %s  %s", issue.Number, clean(issue.Title), criteriaSummary(issue.Criteria))
		lines = append(lines, line{text: text, url: issue.URL})
	}
	return lines
}

// pullRequestStatus describes only what GitHub reported. Values it doesn't know
// are shown as they came, instead of guessing what they mean.
func pullRequestStatus(pr pending.PullRequest) string {
	status := ciStatus(pr.CIState) + ", " + reviewStatus(pr.ReviewDecision)
	if pr.Draft {
		return status + ", draft"
	}
	return status
}

func ciStatus(state string) string {
	switch state {
	case "":
		return "no CI"
	case "SUCCESS":
		return "CI passed"
	case "FAILURE", "ERROR":
		return "CI failed"
	case "PENDING", "EXPECTED":
		return "CI running"
	}
	return "CI " + clean(state)
}

func reviewStatus(decision string) string {
	switch decision {
	case "":
		return "no review decision"
	case stateApproved:
		return "approved"
	case stateChangesRequested:
		return "changes requested"
	case "REVIEW_REQUIRED":
		return "review required"
	}
	return clean(decision)
}
