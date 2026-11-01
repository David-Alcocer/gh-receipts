package criteria

import (
	"context"
	"errors"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

// issueQuery reads one issue. Asking for a pull request's number fails with
// NOT_FOUND, because the field only resolves issues.
const issueQuery = `query Issue($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      number
      title
      state
      url
      body
    }
  }
}`

const (
	errorTypeNotFound = "NOT_FOUND"
	issuePath         = "repository.issue"
)

// Issue is an issue with its parsed criteria.
type Issue struct {
	Number int
	Title  string
	State  string
	URL    string
	Result
}

// Fetch reads an issue and parses its criteria.
func Fetch(ctx context.Context, repo repository.Repository, number int) (Issue, error) {
	client, err := ghclient.New()
	if err != nil {
		return Issue{}, err
	}
	return FetchWith(ctx, client, repo, number)
}

// FetchWith is Fetch with a given client.
func FetchWith(ctx context.Context, client activity.GraphQLClient, repo repository.Repository, number int) (Issue, error) {
	var response struct {
		Repository struct {
			Issue *struct {
				Number int
				Title  string
				State  string
				URL    string
				Body   string
			}
		}
	}
	variables := map[string]interface{}{"owner": repo.Owner, "name": repo.Name, "number": number}
	if err := client.DoWithContext(ctx, issueQuery, variables, &response); err != nil {
		var graphQLError *api.GraphQLError
		if errors.As(err, &graphQLError) && graphQLError.Match(errorTypeNotFound, issuePath) {
			return Issue{}, notFoundError(repo, number)
		}
		return Issue{}, fmt.Errorf("read issue %d of %s/%s: %w", number, repo.Owner, repo.Name, err)
	}

	issue := response.Repository.Issue
	if issue == nil {
		return Issue{}, notFoundError(repo, number)
	}
	return Issue{
		Number: issue.Number,
		Title:  issue.Title,
		State:  issue.State,
		URL:    issue.URL,
		Result: Parse(issue.Body),
	}, nil
}

func notFoundError(repo repository.Repository, number int) error {
	return fmt.Errorf("issue %d of %s/%s not found: check the number, and that it isn't a pull request", number, repo.Owner, repo.Name)
}
