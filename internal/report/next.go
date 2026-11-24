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
		PullRequests:   pullRequestDocuments(p.PullRequests),
		ReviewRequests: nonNil(p.ReviewRequests),
		Issues:         issueDocuments(p.Issues),
		Warnings:       nonNil(p.Warnings),
	})
	if err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

// pullRequestDocuments and issueDocuments keep the promise that lists are []
// and never null, also inside each item.
func pullRequestDocuments(prs []pending.PullRequest) []pending.PullRequest {
	documents := make([]pending.PullRequest, 0, len(prs))
	for _, pr := range prs {
		pr.Closes = nonNil(pr.Closes)
		documents = append(documents, pr)
	}
	return documents
}

func issueDocuments(issues []pending.Issue) []pending.Issue {
	documents := make([]pending.Issue, 0, len(issues))
	for _, issue := range issues {
		issue.Labels = nonNil(issue.Labels)
		issue.BlockedBy = nonNil(issue.BlockedBy)
		documents = append(documents, issue)
	}
	return documents
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
			lines:   openPullRequestLines(repo, p.PullRequests),
			moreURL: base + "/pulls?q=" + url.QueryEscape("is:pr is:open author:@me"),
		},
		reviewRequestSection(repo, p.ReviewRequests),
		{
			title:   "ISSUES ASSIGNED TO YOU",
			lines:   assignedIssueLines(repo, p.Issues),
			moreURL: base + "/issues?q=" + url.QueryEscape("is:issue is:open assignee:@me"),
		},
	}
	return slices.DeleteFunc(all, func(s section) bool { return len(s.lines) == 0 })
}

func repositoryURL(repo repository.Repository) string {
	return fmt.Sprintf("https://%s/%s/%s", repo.Host, repo.Owner, repo.Name)
}

func openPullRequestLines(repo repository.Repository, prs []pending.PullRequest) []line {
	lines := make([]line, 0, len(prs))
	for _, pr := range prs {
		text := fmt.Sprintf("#%d %s  %s", pr.Number, clean(pr.Title), pullRequestStatus(repo, pr))
		lines = append(lines, line{text: text, url: pr.URL})
	}
	return lines
}

// assignedIssueLines adds labels as they are named and the blockers still open,
// so an agent can rank the issues with its team's own rules.
func assignedIssueLines(repo repository.Repository, issues []pending.Issue) []line {
	lines := make([]line, 0, len(issues))
	for _, issue := range issues {
		parts := []string{fmt.Sprintf("#%d %s", issue.Number, clean(issue.Title)), criteriaSummary(issue.Criteria)}
		if len(issue.Labels) > 0 {
			labels := make([]string, 0, len(issue.Labels))
			for _, label := range issue.Labels {
				labels = append(labels, clean(label))
			}
			parts = append(parts, strings.Join(labels, ", "))
		}
		if open := openDependencies(repo, issue.BlockedBy); len(open) > 0 {
			parts = append(parts, "blocked by "+strings.Join(open, ", "))
		}
		lines = append(lines, line{text: strings.Join(parts, "  "), url: issue.URL})
	}
	return lines
}

// pullRequestStatus describes only what GitHub reported. Values it doesn't know
// are shown as they came, instead of guessing what they mean.
func pullRequestStatus(repo repository.Repository, pr pending.PullRequest) string {
	parts := []string{ciStatus(pr.CIState), reviewStatus(pr.ReviewDecision)}
	if pr.Draft {
		parts = append(parts, "draft")
	}
	if merge := mergeStatus(pr.MergeState); merge != "" {
		parts = append(parts, merge)
	}
	if len(pr.Closes) > 0 {
		parts = append(parts, "closes "+strings.Join(dependencyRefs(repo, pr.Closes), ", "))
	}
	return strings.Join(parts, ", ")
}

// mergeStatus describes GitHub's mergeStateStatus. UNKNOWN means GitHub is still
// computing it, and DRAFT is already said, so neither adds anything.
func mergeStatus(state string) string {
	switch state {
	case "", "UNKNOWN", "DRAFT":
		return ""
	case "BEHIND":
		return "behind base branch"
	case "DIRTY":
		return "has conflicts"
	case "BLOCKED":
		return "blocked by branch rules"
	case "UNSTABLE":
		return "checks not passing"
	case "CLEAN", "HAS_HOOKS":
		return "ready to merge"
	}
	return clean(state)
}

func openDependencies(repo repository.Repository, dependencies []pending.Dependency) []string {
	var open []pending.Dependency
	for _, d := range dependencies {
		if d.State == "OPEN" {
			open = append(open, d)
		}
	}
	return dependencyRefs(repo, open)
}

// dependencyRefs writes #N for issues in repo and owner/name#N for the rest.
func dependencyRefs(repo repository.Repository, dependencies []pending.Dependency) []string {
	refs := make([]string, 0, len(dependencies))
	for _, d := range dependencies {
		ref := fmt.Sprintf("#%d", d.Number)
		if d.Repository != "" && d.Repository != repo.Owner+"/"+repo.Name {
			ref = clean(d.Repository) + ref
		}
		refs = append(refs, ref)
	}
	return refs
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
