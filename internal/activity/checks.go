package activity

import (
	"context"
	"slices"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
)

const (
	checkRunType       = "CheckRun"
	rollupStateSuccess = "SUCCESS"
)

var (
	// failedCheckRunConclusions leaves out CANCELLED: a cancelled run usually
	// means someone stopped it or a newer commit replaced it.
	failedCheckRunConclusions = []string{"FAILURE", "TIMED_OUT", "STARTUP_FAILURE"}
	failedStatusStates        = []string{"FAILURE", "ERROR"}
)

// checkContext is a CheckRun (Actions and other check apps) or a StatusContext
// (the older commit status API). The two have different fields.
type checkContext struct {
	Typename   string `json:"__typename"`
	Name       string
	Conclusion string
	URL        string
	Context    string
	State      string
	TargetURL  string `json:"targetUrl"`
}

type contextPage struct {
	PageInfo struct {
		HasNextPage bool
		EndCursor   string
	}
	Nodes []checkContext
}

type commitNode struct {
	ID                string
	OID               string
	URL               string
	StatusCheckRollup *struct {
		State    string
		Contexts contextPage
	}
}

// branchChecks is the default branch and the failed checks on it.
type branchChecks struct {
	Branch string
	Failed []FailedCheck
}

type commitPage struct {
	branch     string
	commits    []commitNode
	nextCursor interface{}
}

// fetchFailedChecks returns the failed checks on default branch commits since
// the given time. It filters by commit date, so a check that fails later on an
// older commit isn't reported.
func fetchFailedChecks(ctx context.Context, client GraphQLClient, repo repository.Repository, since time.Time) (branchChecks, error) {
	var result branchChecks
	var cursor interface{}
	for {
		page, err := fetchCommits(ctx, client, repo, since, cursor)
		if err != nil {
			return branchChecks{}, err
		}
		result.Branch = page.branch
		for _, commit := range page.commits {
			checks, err := failedChecksOn(ctx, client, commit)
			if err != nil {
				return branchChecks{}, err
			}
			result.Failed = slices.Concat(result.Failed, checks)
		}
		if page.nextCursor == nil {
			return result, nil
		}
		cursor = page.nextCursor
	}
}

func fetchCommits(ctx context.Context, client GraphQLClient, repo repository.Repository, since time.Time, cursor interface{}) (commitPage, error) {
	var response struct {
		Repository struct {
			DefaultBranchRef *struct {
				Name   string
				Target struct {
					History struct {
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
						Nodes []commitNode
					}
				}
			}
		}
	}
	variables := map[string]interface{}{
		"owner":  repo.Owner,
		"name":   repo.Name,
		"since":  since.UTC().Format(time.RFC3339),
		"cursor": cursor,
	}
	if err := client.DoWithContext(ctx, checksQuery, variables, &response); err != nil {
		return commitPage{}, err
	}

	// An empty repository has no default branch.
	branch := response.Repository.DefaultBranchRef
	if branch == nil {
		return commitPage{}, nil
	}
	history := branch.Target.History
	page := commitPage{branch: branch.Name, commits: history.Nodes}
	if history.PageInfo.HasNextPage {
		page.nextCursor = history.PageInfo.EndCursor
	}
	return page, nil
}

func failedChecksOn(ctx context.Context, client GraphQLClient, commit commitNode) ([]FailedCheck, error) {
	rollup := commit.StatusCheckRollup
	if rollup == nil || rollup.State == rollupStateSuccess {
		return nil, nil
	}

	var failed []FailedCheck
	page := rollup.Contexts
	for {
		for _, check := range page.Nodes {
			if parsed, ok := failedCheck(check, commit); ok {
				failed = append(failed, parsed)
			}
		}
		if !page.PageInfo.HasNextPage {
			return failed, nil
		}

		var response struct {
			Node struct {
				StatusCheckRollup struct{ Contexts contextPage }
			}
		}
		variables := map[string]interface{}{"id": commit.ID, "cursor": page.PageInfo.EndCursor}
		if err := client.DoWithContext(ctx, checkContextsPageQuery, variables, &response); err != nil {
			return nil, err
		}
		page = response.Node.StatusCheckRollup.Contexts
	}
}

func failedCheck(check checkContext, commit commitNode) (FailedCheck, bool) {
	if check.Typename == checkRunType {
		if !slices.Contains(failedCheckRunConclusions, check.Conclusion) {
			return FailedCheck{}, false
		}
		return FailedCheck{Name: check.Name, Conclusion: check.Conclusion, CommitSHA: commit.OID, URL: check.URL}, true
	}

	if !slices.Contains(failedStatusStates, check.State) {
		return FailedCheck{}, false
	}
	// A commit status can have no target URL, and every item needs a source.
	url := check.TargetURL
	if url == "" {
		url = commit.URL
	}
	return FailedCheck{Name: check.Context, Conclusion: check.State, CommitSHA: commit.OID, URL: url}, true
}
