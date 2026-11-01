package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/project"
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
	// configuredProject returns the repository's project number, or 0.
	configuredProject func(ctx context.Context, repo repository.Repository) (int, error)
	fetchProject      func(ctx context.Context, owner string, number int, since time.Time, viewer string) (project.Project, error)
}

type todayFlags struct {
	since   string
	repo    string
	project int
	asJSON  bool
}

func defaultTodayDeps() todayDeps {
	return todayDeps{
		store:             session.Store{},
		now:               time.Now,
		currentRepo:       repository.Current,
		fetch:             activity.Fetch,
		configuredProject: project.ConfiguredNumber,
		fetchProject:      project.Fetch,
	}
}

func newTodayCmd(deps todayDeps) *cobra.Command {
	var flags todayFlags
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show what changed since your last session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("project") && flags.project <= 0 {
				return fmt.Errorf("invalid --project %d: it must be a positive number", flags.project)
			}
			return runToday(cmd, deps, flags)
		},
	}
	cmd.Flags().StringVar(&flags.since, "since", "", `start from a duration ago ("48h") or a date ("2026-11-01"), keeping the last session`)
	cmd.Flags().StringVar(&flags.repo, "repo", "", "repository to report on, as owner/name")
	cmd.Flags().IntVar(&flags.project, "project", 0, "GitHub Project number, instead of the one in .github/receipts.yml")
	cmd.Flags().BoolVar(&flags.asJSON, "json", false, "print the complete data as JSON, keeping the last session")
	return cmd
}

func runToday(cmd *cobra.Command, deps todayDeps, flags todayFlags) error {
	// The run starts now, so the next one includes anything that happens while
	// this one is fetching.
	start := deps.now()

	repo, err := resolveRepo(deps.currentRepo, flags.repo)
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
	board, boardRead, err := fetchBoard(cmd.Context(), deps, flags.project, repo, since, result.Viewer)
	if err != nil {
		return err
	}
	if !boardRead {
		result.Warnings = append(result.Warnings, project.ErrMissingScope.Error())
	}
	for _, warning := range result.Warnings {
		warn(cmd, warning)
	}

	info := report.Info{Repo: repo, Since: since, SinceSource: source, Project: board}
	if flags.asJSON {
		return report.JSON(cmd.OutOrStdout(), result, info)
	}
	if err := report.Text(cmd.OutOrStdout(), result, info); err != nil {
		return err
	}

	// Only a report someone read moves the session forward. Otherwise an agent
	// reading --json, a look back with --since, or a board that couldn't be read
	// would hide news from the next report.
	if flags.since == "" && boardRead {
		if err := deps.store.Save(repo, start); err != nil {
			warn(cmd, err.Error())
		}
	}
	return nil
}

// fetchBoard returns the selected project, or nil when there is none. A missing
// scope isn't an error: the configuration is shared, and a teammate without the
// scope should still get the rest of the report. It returns false in that case.
func fetchBoard(ctx context.Context, deps todayDeps, number int, repo repository.Repository, since time.Time, viewer string) (*project.Project, bool, error) {
	if number == 0 {
		configured, err := deps.configuredProject(ctx, repo)
		if err != nil {
			return nil, false, err
		}
		number = configured
	}
	if number == 0 {
		return nil, true, nil
	}

	board, err := deps.fetchProject(ctx, repo.Owner, number, since, viewer)
	if errors.Is(err, project.ErrMissingScope) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &board, true, nil
}

func resolveRepo(currentRepo func() (repository.Repository, error), flag string) (repository.Repository, error) {
	if flag != "" {
		repo, err := repository.Parse(flag)
		if err != nil {
			return repository.Repository{}, fmt.Errorf("invalid --repo: %w", err)
		}
		return repo, nil
	}

	repo, err := currentRepo()
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
