package activity

import (
	"context"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
)

type reviewRequestNode struct {
	Number int
	Title  string
	URL    string
	Author *actor
}

type reviewRequests struct {
	Viewer       string
	PullRequests []PullRequest
	Warnings     []string
}

// fetchReviewRequests returns the authenticated user and the open pull requests
// requesting their review, directly or through a team.
func fetchReviewRequests(ctx context.Context, client GraphQLClient, repo repository.Repository) (reviewRequests, error) {
	search := fmt.Sprintf("repo:%s/%s is:pr is:open review-requested:@me", repo.Owner, repo.Name)

	var viewer string
	nodes, warnings, err := searchAll(search, func(cursor interface{}) (searchConnection[reviewRequestNode], error) {
		var response struct {
			Viewer struct{ Login string }
			Search searchConnection[reviewRequestNode]
		}
		variables := map[string]interface{}{"search": search, "cursor": cursor}
		err := client.DoWithContext(ctx, reviewRequestsQuery, variables, &response)
		viewer = response.Viewer.Login
		return response.Search, err
	})
	if err != nil {
		return reviewRequests{}, err
	}

	var pullRequests []PullRequest
	for _, node := range nodes {
		if node.Author.isBot() {
			continue
		}
		pullRequests = append(pullRequests, PullRequest{
			Number: node.Number,
			Title:  node.Title,
			Author: node.Author.login(),
			URL:    node.URL,
		})
	}
	return reviewRequests{Viewer: viewer, PullRequests: pullRequests, Warnings: warnings}, nil
}
