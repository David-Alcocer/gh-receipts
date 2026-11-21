// Package activity fetches facts about a repository from the GitHub GraphQL API.
package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

// Activity is what happened in a repository since a point in time. Items
// authored by bots are excluded.
type Activity struct {
	Viewer string
	// DefaultBranch is empty for a repository without commits.
	DefaultBranch  string
	MergedPRs      []PullRequest
	OpenedPRs      []PullRequest
	ReviewRequests []PullRequest
	Reviews        []Review
	Comments       []Comment
	// Issues are the issues updated since the start of the window, so the
	// report can tell whose they are. IssueEvents are their changes.
	Issues       []Issue
	IssueEvents  []IssueEvent
	FailedChecks []FailedCheck
	// Warnings describes results that may be incomplete.
	Warnings []string
}

// Issue is an issue and who it is assigned to now.
type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	State     string   `json:"state"`
	Assignees []string `json:"assignees"`
}

// Kinds of IssueEvent.
const (
	EventClosed       = "closed"
	EventReopened     = "reopened"
	EventAssigned     = "assigned"
	EventUnassigned   = "unassigned"
	EventRenamed      = "renamed"
	EventLabeled      = "labeled"
	EventUnlabeled    = "unlabeled"
	EventMilestoned   = "milestoned"
	EventDemilestoned = "demilestoned"
	EventEdited       = "edited"
)

// IssueEvent is a change to an issue made by a person. Detail depends on Kind:
// the state reason for closed, the assignee for assigned and unassigned, the
// new title for renamed, the label or milestone name, and empty otherwise.
type IssueEvent struct {
	IssueNumber int       `json:"issueNumber"`
	Kind        string    `json:"kind"`
	Actor       string    `json:"actor"`
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"createdAt"`
	// URL is the event itself when GitHub gives it one, and the issue otherwise.
	URL string `json:"url"`
}

// PullRequest is a pull request and its source.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"`
}

// Review is a submitted pull request review.
type Review struct {
	PRNumber    int       `json:"prNumber"`
	PRAuthor    string    `json:"prAuthor"`
	Reviewer    string    `json:"reviewer"`
	State       string    `json:"state"`
	Excerpt     string    `json:"excerpt"`
	SubmittedAt time.Time `json:"submittedAt"`
	URL         string    `json:"url"`
}

// Comment is a conversation comment on an issue or pull request, or an inline
// comment on a pull request's code.
type Comment struct {
	Author       string    `json:"author"`
	Excerpt      string    `json:"excerpt"`
	CreatedAt    time.Time `json:"createdAt"`
	URL          string    `json:"url"`
	ParentNumber int       `json:"parentNumber"`
	ParentAuthor string    `json:"parentAuthor"`
	// OnPullRequest is false when the parent is an issue.
	OnPullRequest bool `json:"onPullRequest"`
	// ReviewURL links an inline comment to the review that contains it. It is
	// empty for conversation comments.
	ReviewURL string `json:"reviewUrl"`
}

// FailedCheck is a failed check on a commit of the default branch.
type FailedCheck struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	CommitSHA  string `json:"commitSha"`
	URL        string `json:"url"`
}

// GraphQLClient is the part of the go-gh client this package uses.
type GraphQLClient interface {
	DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error
}

// Fetch returns the activity in repo since the given time.
func Fetch(ctx context.Context, repo repository.Repository, since time.Time) (Activity, error) {
	client, err := ghclient.New()
	if err != nil {
		return Activity{}, err
	}
	return FetchWith(ctx, client, repo, since)
}

// FetchWith is Fetch with a given client.
func FetchWith(ctx context.Context, client GraphQLClient, repo repository.Repository, since time.Time) (Activity, error) {
	result, err := fetchUpdated(ctx, client, repo, since)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch updated issues and pull requests: %w", err)
	}
	requests, err := FetchReviewRequests(ctx, client, repo)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch review requests: %w", err)
	}
	checks, err := fetchFailedChecks(ctx, client, repo, since)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch failed checks: %w", err)
	}

	result.Viewer = requests.Viewer
	result.ReviewRequests = requests.PullRequests
	result.DefaultBranch = checks.Branch
	result.FailedChecks = checks.Failed
	result.Warnings = append(result.Warnings, requests.Warnings...)
	return result, nil
}
