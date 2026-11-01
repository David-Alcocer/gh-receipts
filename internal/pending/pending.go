// Package pending finds the open work waiting for the authenticated user in a
// repository. It doesn't depend on any session: what's pending doesn't change
// with the last time someone looked.
package pending

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/criteria"
	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

const (
	ciFailure              = "FAILURE"
	ciError                = "ERROR"
	reviewChangesRequested = "CHANGES_REQUESTED"
	needsActionRank        = 0
	otherPullRequestRank   = 1
)

// Pending is the viewer's open work. Lists follow the documented order: see
// docs/next-json.md.
type Pending struct {
	Viewer         string
	PullRequests   []PullRequest
	ReviewRequests []activity.PullRequest
	Issues         []Issue
	// Warnings describes results that may be incomplete.
	Warnings []string
}

// PullRequest is an open pull request by the viewer. CIState and ReviewDecision
// are GitHub's values, and empty when GitHub has none.
type PullRequest struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	Draft          bool   `json:"draft"`
	CIState        string `json:"ciState"`
	ReviewDecision string `json:"reviewDecision"`
}

// Issue is an open issue assigned to the viewer.
type Issue struct {
	Number   int             `json:"number"`
	Title    string          `json:"title"`
	URL      string          `json:"url"`
	Criteria criteria.Result `json:"criteria"`
}

type pullRequestNode struct {
	Number         int
	Title          string
	URL            string
	IsDraft        bool
	ReviewDecision string
	Commits        struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct{ State string }
			}
		}
	}
}

type issueNode struct {
	Number int
	Title  string
	URL    string
	Body   string
}

// Fetch returns the viewer's open work in repo.
func Fetch(ctx context.Context, repo repository.Repository) (Pending, error) {
	client, err := ghclient.New()
	if err != nil {
		return Pending{}, err
	}
	return FetchWith(ctx, client, repo)
}

// FetchWith is Fetch with a given client.
func FetchWith(ctx context.Context, client activity.GraphQLClient, repo repository.Repository) (Pending, error) {
	if err := checkRepository(ctx, client, repo); err != nil {
		return Pending{}, err
	}
	requests, err := activity.FetchReviewRequests(ctx, client, repo)
	if err != nil {
		return Pending{}, fmt.Errorf("fetch review requests: %w", err)
	}
	pullRequests, pullRequestWarnings, err := fetchOpenPullRequests(ctx, client, repo)
	if err != nil {
		return Pending{}, fmt.Errorf("fetch open pull requests: %w", err)
	}
	issues, issueWarnings, err := fetchAssignedIssues(ctx, client, repo)
	if err != nil {
		return Pending{}, fmt.Errorf("fetch assigned issues: %w", err)
	}

	slices.SortFunc(requests.PullRequests, func(a, b activity.PullRequest) int { return cmp.Compare(a.Number, b.Number) })
	sortPullRequests(pullRequests)
	slices.SortFunc(issues, func(a, b Issue) int { return cmp.Compare(a.Number, b.Number) })
	return Pending{
		Viewer:         requests.Viewer,
		PullRequests:   pullRequests,
		ReviewRequests: requests.PullRequests,
		Issues:         issues,
		Warnings:       slices.Concat(requests.Warnings, pullRequestWarnings, issueWarnings),
	}, nil
}

func checkRepository(ctx context.Context, client activity.GraphQLClient, repo repository.Repository) error {
	var response struct{}
	variables := map[string]interface{}{"owner": repo.Owner, "name": repo.Name}
	if err := client.DoWithContext(ctx, repositoryExistsQuery, variables, &response); err != nil {
		return fmt.Errorf("read repository %s/%s: %w", repo.Owner, repo.Name, err)
	}
	return nil
}

func fetchOpenPullRequests(ctx context.Context, client activity.GraphQLClient, repo repository.Repository) ([]PullRequest, []string, error) {
	search := fmt.Sprintf("repo:%s/%s is:pr is:open author:@me", repo.Owner, repo.Name)
	nodes, warnings, err := activity.SearchAll(search, func(cursor interface{}) (activity.SearchConnection[pullRequestNode], error) {
		return searchPage[pullRequestNode](ctx, client, openPullRequestsQuery, search, cursor)
	})
	if err != nil {
		return nil, nil, err
	}

	pullRequests := make([]PullRequest, 0, len(nodes))
	for _, node := range nodes {
		pullRequests = append(pullRequests, PullRequest{
			Number:         node.Number,
			Title:          node.Title,
			URL:            node.URL,
			Draft:          node.IsDraft,
			CIState:        ciState(node),
			ReviewDecision: node.ReviewDecision,
		})
	}
	return pullRequests, warnings, nil
}

func fetchAssignedIssues(ctx context.Context, client activity.GraphQLClient, repo repository.Repository) ([]Issue, []string, error) {
	search := fmt.Sprintf("repo:%s/%s is:issue is:open assignee:@me", repo.Owner, repo.Name)
	nodes, warnings, err := activity.SearchAll(search, func(cursor interface{}) (activity.SearchConnection[issueNode], error) {
		return searchPage[issueNode](ctx, client, assignedIssuesQuery, search, cursor)
	})
	if err != nil {
		return nil, nil, err
	}

	issues := make([]Issue, 0, len(nodes))
	for _, node := range nodes {
		issues = append(issues, Issue{Number: node.Number, Title: node.Title, URL: node.URL, Criteria: criteria.Parse(node.Body)})
	}
	return issues, warnings, nil
}

func searchPage[T any](ctx context.Context, client activity.GraphQLClient, query, search string, cursor interface{}) (activity.SearchConnection[T], error) {
	var response struct {
		Search activity.SearchConnection[T]
	}
	variables := map[string]interface{}{"search": search, "cursor": cursor}
	err := client.DoWithContext(ctx, query, variables, &response)
	return response.Search, err
}

// ciState reads the checks of the latest commit. A pull request has none when
// nothing runs on that commit.
func ciState(node pullRequestNode) string {
	commits := node.Commits.Nodes
	if len(commits) == 0 || commits[0].Commit.StatusCheckRollup == nil {
		return ""
	}
	return commits[0].Commit.StatusCheckRollup.State
}

// sortPullRequests puts the ones waiting on their author first: failed checks or
// requested changes. Numbers order them within that, oldest first.
func sortPullRequests(pullRequests []PullRequest) {
	slices.SortFunc(pullRequests, func(a, b PullRequest) int {
		return cmp.Or(cmp.Compare(rank(a), rank(b)), cmp.Compare(a.Number, b.Number))
	})
}

func rank(pr PullRequest) int {
	if pr.CIState == ciFailure || pr.CIState == ciError || pr.ReviewDecision == reviewChangesRequested {
		return needsActionRank
	}
	return otherPullRequestRank
}
