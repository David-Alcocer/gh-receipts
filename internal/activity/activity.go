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
	Viewer         string
	MergedPRs      []PullRequest
	OpenedPRs      []PullRequest
	ReviewRequests []PullRequest
	Reviews        []Review
	Comments       []Comment
	FailedChecks   []FailedCheck
	// Warnings describes results that may be incomplete.
	Warnings []string
}

// PullRequest is a pull request and its source.
type PullRequest struct {
	Number int
	Title  string
	Author string
	URL    string
}

// Review is a submitted pull request review.
type Review struct {
	PRNumber    int
	Reviewer    string
	State       string
	SubmittedAt time.Time
	URL         string
}

// Comment is a conversation comment on an issue or pull request, or an inline
// comment on a pull request's code.
type Comment struct {
	Author       string
	Excerpt      string
	CreatedAt    time.Time
	URL          string
	ParentNumber int
	ParentAuthor string
	// ReviewURL links an inline comment to the review that contains it. It is
	// empty for conversation comments.
	ReviewURL string
}

// FailedCheck is a failed check on a commit of the default branch.
type FailedCheck struct {
	Name       string
	Conclusion string
	CommitSHA  string
	URL        string
}

// graphQLClient is the part of the go-gh client this package uses.
type graphQLClient interface {
	DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error
}

// Fetch returns the activity in repo since the given time.
func Fetch(ctx context.Context, repo repository.Repository, since time.Time) (Activity, error) {
	client, err := ghclient.New()
	if err != nil {
		return Activity{}, err
	}
	return fetch(ctx, client, repo, since)
}

func fetch(ctx context.Context, client graphQLClient, repo repository.Repository, since time.Time) (Activity, error) {
	result, err := fetchUpdated(ctx, client, repo, since)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch updated issues and pull requests: %w", err)
	}
	requests, err := fetchReviewRequests(ctx, client, repo)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch review requests: %w", err)
	}
	failedChecks, err := fetchFailedChecks(ctx, client, repo, since)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch failed checks: %w", err)
	}

	result.Viewer = requests.Viewer
	result.ReviewRequests = requests.PullRequests
	result.FailedChecks = failedChecks
	result.Warnings = append(result.Warnings, requests.Warnings...)
	return result, nil
}
