package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/pending"
	"github.com/David-Alcocer/gh-receipts/internal/project"
	"github.com/David-Alcocer/gh-receipts/internal/report"
)

// defaultContextTimeout is shorter than the 30 seconds docs/agent-hook.md gives
// the hook, so the agent gets what was read instead of nothing.
const defaultContextTimeout = 20 * time.Second

// contextDeps reuses today's dependencies to read the changes, but context never
// calls the store's Save.
type contextDeps struct {
	today        todayDeps
	fetchPending func(ctx context.Context, repo repository.Repository) (pending.Pending, error)
	timeout      time.Duration
}

type changesResult struct {
	activity activity.Activity
	board    *project.Project
	err      error
}

type pendingResult struct {
	pending pending.Pending
	err     error
}

func defaultContextDeps() contextDeps {
	return contextDeps{
		today:        defaultTodayDeps(),
		fetchPending: pending.Fetch,
		timeout:      defaultContextTimeout,
	}
}

func newContextCmd(deps contextDeps) *cobra.Command {
	var repoFlag string
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Show a short summary for a coding agent's session start",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContext(cmd, deps, repoFlag)
		},
	}
	cmd.Flags().StringVar(&repoFlag, "repo", "", "repository to report on, as owner/name")
	return cmd
}

// runContext runs from an agent's session-start hook, so a problem costs one
// line of output, never a failed command. Everything goes to stdout, because a
// hook may only pass stdout to the agent.
func runContext(cmd *cobra.Command, deps contextDeps, repoFlag string) error {
	out := cmd.OutOrStdout()
	repo, err := resolveRepo(deps.today.currentRepo, repoFlag)
	if err != nil {
		return writeContextFailure(out, err)
	}
	since, source, sessionErr := lastSession(deps.today.store, repo, deps.today.now())

	ctx, cancel := context.WithTimeout(cmd.Context(), deps.timeout)
	defer cancel()
	changes, work := fetchContext(ctx, deps, repo, since)
	if changes.err != nil && work.err != nil {
		return writeContextFailure(out, fmt.Errorf("couldn't read GitHub: %w", explainFetchError(work.err, deps.timeout)))
	}

	c := report.Context{Info: report.Info{Repo: repo, Since: since, SinceSource: source}}
	if sessionErr != nil {
		c.Notes = append(c.Notes, sessionErr.Error())
	}
	if changes.err != nil {
		c.Notes = append(c.Notes, "couldn't read what changed: "+explainFetchError(changes.err, deps.timeout).Error())
	} else {
		c.Activity, c.Info.Project = &changes.activity, changes.board
		c.Notes = append(c.Notes, changes.activity.Warnings...)
	}
	if work.err != nil {
		c.Notes = append(c.Notes, "couldn't read what is waiting for you: "+explainFetchError(work.err, deps.timeout).Error())
	} else {
		c.Pending = &work.pending
		c.Notes = append(c.Notes, work.pending.Warnings...)
	}
	return report.ContextText(out, c)
}

// fetchContext reads the changes and the pending work at the same time, so the
// timeout covers the slower of the two instead of their sum.
func fetchContext(ctx context.Context, deps contextDeps, repo repository.Repository, since time.Time) (changesResult, pendingResult) {
	var changes changesResult
	var work pendingResult
	var wg sync.WaitGroup
	wg.Go(func() {
		result, board, _, err := fetchChanges(ctx, deps.today, 0, repo, since)
		changes = changesResult{activity: result, board: board, err: err}
	})
	wg.Go(func() {
		result, err := deps.fetchPending(ctx, repo)
		work = pendingResult{pending: result, err: err}
	})
	wg.Wait()
	return changes, work
}

func explainFetchError(err error, timeout time.Duration) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("GitHub took more than %s to answer", timeout)
	}
	return err
}

func writeContextFailure(w io.Writer, err error) error {
	_, writeErr := fmt.Fprintf(w, "gh receipts context: %s\n", err)
	return writeErr
}
