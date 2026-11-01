package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/report"
	"github.com/David-Alcocer/gh-receipts/internal/session"
)

const (
	firstRunWindow = 24 * time.Hour
	dateLayout     = time.DateOnly
)

type lastRunStore interface {
	LastRun(repo repository.Repository) (time.Time, bool, error)
	Save(repo repository.Repository, at time.Time) error
}

// todayDeps holds everything the command needs from the outside world.
type todayDeps struct {
	store       lastRunStore
	now         func() time.Time
	currentRepo func() (repository.Repository, error)
	fetch       func(ctx context.Context, repo repository.Repository, since time.Time) (activity.Activity, error)
}

type todayFlags struct {
	since  string
	repo   string
	asJSON bool
}

func defaultTodayDeps() todayDeps {
	return todayDeps{
		store:       session.Store{},
		now:         time.Now,
		currentRepo: repository.Current,
		fetch:       activity.Fetch,
	}
}

func newTodayCmd(deps todayDeps) *cobra.Command {
	var flags todayFlags
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show what changed since your last session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToday(cmd, deps, flags)
		},
	}
	cmd.Flags().StringVar(&flags.since, "since", "", `start from a duration ago ("48h") or a date ("2026-11-01"), keeping the last session`)
	cmd.Flags().StringVar(&flags.repo, "repo", "", "repository to report on, as owner/name")
	cmd.Flags().BoolVar(&flags.asJSON, "json", false, "print the complete data as JSON, keeping the last session")
	return cmd
}

func runToday(cmd *cobra.Command, deps todayDeps, flags todayFlags) error {
	// The run starts now, so the next one includes anything that happens while
	// this one is fetching.
	start := deps.now()

	repo, err := resolveRepo(deps, flags.repo)
	if err != nil {
		return err
	}
	since, source, err := resolveSince(cmd, deps, repo, flags.since, start)
	if err != nil {
		return err
	}

	result, err := deps.fetch(cmd.Context(), repo, since)
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		warn(cmd, warning)
	}

	info := report.Info{Repo: repo, Since: since, SinceSource: source}
	if flags.asJSON {
		return report.JSON(cmd.OutOrStdout(), result, info)
	}
	if err := report.Text(cmd.OutOrStdout(), result, info); err != nil {
		return err
	}

	// Only a report someone read moves the session forward. Otherwise an agent
	// reading --json, or a look back with --since, would hide news from the next
	// report.
	if flags.since == "" {
		if err := deps.store.Save(repo, start); err != nil {
			warn(cmd, err.Error())
		}
	}
	return nil
}

func resolveRepo(deps todayDeps, flag string) (repository.Repository, error) {
	if flag != "" {
		repo, err := repository.Parse(flag)
		if err != nil {
			return repository.Repository{}, fmt.Errorf("invalid --repo: %w", err)
		}
		return repo, nil
	}

	repo, err := deps.currentRepo()
	if err != nil {
		return repository.Repository{}, errors.New("no GitHub repository found here: run this inside a clone or pass --repo owner/name")
	}
	return repo, nil
}

func resolveSince(cmd *cobra.Command, deps todayDeps, repo repository.Repository, flag string, now time.Time) (time.Time, string, error) {
	if flag != "" {
		since, err := parseSince(flag, now)
		return since, "--since", err
	}

	last, found, err := deps.store.LastRun(repo)
	if err != nil {
		warn(cmd, err.Error())
		return now.Add(-firstRunWindow), "last 24 hours", nil
	}
	if !found {
		return now.Add(-firstRunWindow), "first run, last 24 hours", nil
	}
	return last.Local(), "last session", nil
}

func parseSince(value string, now time.Time) (time.Time, error) {
	since, err := sinceFromValue(value, now)
	if err != nil {
		return time.Time{}, fmt.Errorf(`invalid --since %q: use a duration like "48h" or a date like "2026-11-01"`, value)
	}
	if since.After(now) {
		return time.Time{}, fmt.Errorf("invalid --since %q: it is in the future", value)
	}
	return since, nil
}

func sinceFromValue(value string, now time.Time) (time.Time, error) {
	if duration, err := time.ParseDuration(value); err == nil {
		return now.Add(-duration), nil
	}
	return time.ParseInLocation(dateLayout, value, time.Local)
}

func warn(cmd *cobra.Command, message string) {
	fmt.Fprintln(cmd.ErrOrStderr(), "warning:", message)
}
