package health

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

// SearchTimeLayout is how times are written in GitHub search qualifiers.
const SearchTimeLayout = "2006-01-02T15:04:05Z"

type author struct {
	Typename string `json:"__typename"`
	Login    string
}

func (a *author) isBot() bool {
	return a != nil && a.Typename == "Bot"
}

type mergedNode struct {
	Number    int
	URL       string
	Additions int
	Deletions int
	CreatedAt time.Time
	MergedAt  time.Time
	Author    *author
	Reviews   struct {
		PageInfo struct{ HasNextPage bool }
		Nodes    []struct {
			SubmittedAt *time.Time
			Author      *author
		}
	}
}

type openNode struct {
	Number    int
	Title     string
	URL       string
	IsDraft   bool
	UpdatedAt time.Time
	Author    *author
}

type issueNode struct {
	Number    int
	CreatedAt time.Time
	ClosedAt  *time.Time
	Author    *author
}

// Fetch reads what Compute needs for the window in repo.
func Fetch(ctx context.Context, repo repository.Repository, window Window, thresholds Thresholds) (Data, error) {
	client, err := ghclient.New()
	if err != nil {
		return Data{}, err
	}
	return FetchWith(ctx, client, repo, window, thresholds)
}

// FetchWith is Fetch with a given client.
func FetchWith(ctx context.Context, client activity.GraphQLClient, repo repository.Repository, window Window, thresholds Thresholds) (Data, error) {
	if err := checkRepository(ctx, client, repo); err != nil {
		return Data{}, err
	}
	merged, mergedWarnings, err := fetchMerged(ctx, client, repo, window.Since)
	if err != nil {
		return Data{}, fmt.Errorf("fetch merged pull requests: %w", err)
	}
	staleBefore := window.Until.Add(-time.Duration(thresholds.StaleDays) * 24 * time.Hour)
	open, openWarnings, err := fetchStale(ctx, client, repo, staleBefore)
	if err != nil {
		return Data{}, fmt.Errorf("fetch stale pull requests: %w", err)
	}
	issues, issueWarnings, err := fetchIssues(ctx, client, repo, window.Since)
	if err != nil {
		return Data{}, fmt.Errorf("fetch issues: %w", err)
	}
	return Data{
		MergedPRs: merged,
		OpenPRs:   open,
		Issues:    issues,
		Warnings:  slices.Concat(mergedWarnings, openWarnings, issueWarnings),
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

func fetchMerged(ctx context.Context, client activity.GraphQLClient, repo repository.Repository, since time.Time) ([]PullRequest, []string, error) {
	search := fmt.Sprintf("repo:%s/%s is:pr is:merged merged:>=%s", repo.Owner, repo.Name, since.UTC().Format(SearchTimeLayout))
	nodes, warnings, err := searchAll[mergedNode](ctx, client, mergedPullRequestsQuery, search)
	if err != nil {
		return nil, nil, err
	}

	var prs []PullRequest
	for _, node := range nodes {
		if node.Author.isBot() {
			continue
		}
		firstReview, found := firstReviewAt(node)
		if !found && node.Reviews.PageInfo.HasNextPage {
			warnings = append(warnings, fmt.Sprintf("only the first 50 reviews of %s were read", node.URL))
		}
		prs = append(prs, PullRequest{
			Number:        node.Number,
			URL:           node.URL,
			Additions:     node.Additions,
			Deletions:     node.Deletions,
			CreatedAt:     node.CreatedAt,
			MergedAt:      node.MergedAt,
			FirstReviewAt: firstReview,
		})
	}
	return prs, warnings, nil
}

// firstReviewAt returns the first submitted review by someone other than the
// author or a bot. Pending reviews have no submission time. A deleted account
// has no author, and still counts as someone else.
func firstReviewAt(node mergedNode) (time.Time, bool) {
	for _, review := range node.Reviews.Nodes {
		if review.SubmittedAt == nil || review.Author.isBot() {
			continue
		}
		if review.Author != nil && node.Author != nil && review.Author.Login == node.Author.Login {
			continue
		}
		return *review.SubmittedAt, true
	}
	return time.Time{}, false
}

// fetchStale narrows the search to pull requests not updated since before, so
// busy repositories don't need every open one.
func fetchStale(ctx context.Context, client activity.GraphQLClient, repo repository.Repository, before time.Time) ([]OpenPullRequest, []string, error) {
	search := fmt.Sprintf("repo:%s/%s is:pr is:open updated:<%s", repo.Owner, repo.Name, before.UTC().Format(SearchTimeLayout))
	nodes, warnings, err := searchAll[openNode](ctx, client, stalePullRequestsQuery, search)
	if err != nil {
		return nil, nil, err
	}

	var prs []OpenPullRequest
	for _, node := range nodes {
		if node.Author.isBot() {
			continue
		}
		prs = append(prs, OpenPullRequest{Number: node.Number, Title: node.Title, URL: node.URL, Draft: node.IsDraft, UpdatedAt: node.UpdatedAt})
	}
	return prs, warnings, nil
}

// fetchIssues reads issues opened and issues closed since a time. An issue in
// both searches is kept once.
func fetchIssues(ctx context.Context, client activity.GraphQLClient, repo repository.Repository, since time.Time) ([]Issue, []string, error) {
	var nodes []issueNode
	var warnings []string
	for _, qualifier := range []string{"created", "closed"} {
		search := fmt.Sprintf("repo:%s/%s is:issue %s:>=%s", repo.Owner, repo.Name, qualifier, since.UTC().Format(SearchTimeLayout))
		found, searchWarnings, err := searchAll[issueNode](ctx, client, issuesQuery, search)
		if err != nil {
			return nil, nil, err
		}
		nodes = slices.Concat(nodes, found)
		warnings = slices.Concat(warnings, searchWarnings)
	}

	byNumber := map[int]Issue{}
	for _, node := range nodes {
		if node.Author.isBot() {
			continue
		}
		issue := Issue{Number: node.Number, CreatedAt: node.CreatedAt}
		if node.ClosedAt != nil {
			issue.ClosedAt = *node.ClosedAt
		}
		byNumber[node.Number] = issue
	}
	issues := slices.Collect(maps.Values(byNumber))
	slices.SortFunc(issues, func(a, b Issue) int { return cmp.Compare(a.Number, b.Number) })
	return issues, warnings, nil
}

func searchAll[T any](ctx context.Context, client activity.GraphQLClient, query, search string) ([]T, []string, error) {
	return activity.SearchAll(search, func(cursor interface{}) (activity.SearchConnection[T], error) {
		var response struct {
			Search activity.SearchConnection[T]
		}
		variables := map[string]interface{}{"search": search, "cursor": cursor}
		err := client.DoWithContext(ctx, query, variables, &response)
		return response.Search, err
	})
}
