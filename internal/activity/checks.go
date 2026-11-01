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

// fetchFailedChecks returns the failed checks on default branch commits since
// the given time. It filters by commit date, so a check that fails later on an
// older commit isn't reported.
func fetchFailedChecks(ctx context.Context, client graphQLClient, repo repository.Repository, since time.Time) ([]FailedCheck, error) {
	var failed []FailedCheck
	var cursor interface{}
	for {
		commits, nextCursor, err := fetchCommits(ctx, client, repo, since, cursor)
		if err != nil {
			return nil, err
		}
		for _, commit := range commits {
			checks, err := failedChecksOn(ctx, client, commit)
			if err != nil {
				return nil, err
			}
			failed = slices.Concat(failed, checks)
		}
		if nextCursor == nil {
			return failed, nil
		}
		cursor = nextCursor
	}
}

func fetchCommits(ctx context.Context, client graphQLClient, repo repository.Repository, since time.Time, cursor interface{}) ([]commitNode, interface{}, error) {
	var response struct {
		Repository struct {
			DefaultBranchRef *struct {
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
		return nil, nil, err
	}

	// An empty repository has no default branch.
	branch := response.Repository.DefaultBranchRef
	if branch == nil {
		return nil, nil, nil
	}
	history := branch.Target.History
	if !history.PageInfo.HasNextPage {
		return history.Nodes, nil, nil
	}
	return history.Nodes, history.PageInfo.EndCursor, nil
}

func failedChecksOn(ctx context.Context, client graphQLClient, commit commitNode) ([]FailedCheck, error) {
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
