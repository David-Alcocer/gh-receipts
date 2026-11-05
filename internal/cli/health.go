package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/David-Alcocer/gh-receipts/internal/config"
	"github.com/David-Alcocer/gh-receipts/internal/health"
	"github.com/David-Alcocer/gh-receipts/internal/report"
)

const defaultHealthDays = 14

// healthDeps holds everything the command needs from the outside world. Like
// next, it doesn't read or change the last session of today.
type healthDeps struct {
	currentRepo func() (repository.Repository, error)
	now         func() time.Time
	readConfig  func(ctx context.Context, repo repository.Repository) (config.Config, error)
	fetch       func(ctx context.Context, repo repository.Repository, window health.Window, thresholds health.Thresholds) (health.Data, error)
}

type healthFlags struct {
	repo   string
	days   int
	asJSON bool
}

func defaultHealthDeps() healthDeps {
	return healthDeps{
		currentRepo: repository.Current,
		now:         time.Now,
		readConfig:  config.Read,
		fetch:       health.Fetch,
	}
}

func newHealthCmd(deps healthDeps) *cobra.Command {
	var flags healthFlags
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show team-level flow signals",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHealth(cmd, deps, flags)
		},
	}
	cmd.Flags().StringVar(&flags.repo, "repo", "", "repository to report on, as owner/name")
	cmd.Flags().IntVar(&flags.days, "days", defaultHealthDays, "number of days to cover, back from now")
	cmd.Flags().BoolVar(&flags.asJSON, "json", false, "print the complete data as JSON")
	return cmd
}

func runHealth(cmd *cobra.Command, deps healthDeps, flags healthFlags) error {
	if flags.days < 1 {
		return fmt.Errorf("--days must be at least 1, got %d", flags.days)
	}
	repo, err := resolveRepo(deps.currentRepo, flags.repo)
	if err != nil {
		return err
	}

	configured, err := deps.readConfig(cmd.Context(), repo)
	if err != nil {
		return err
	}
	thresholds := health.ThresholdsFrom(configured.Health)
	// Whole seconds keep the JSON times short; search qualifiers drop the rest.
	now := deps.now().Truncate(time.Second)
	window := health.Window{Since: now.AddDate(0, 0, -flags.days), Until: now}

	data, err := deps.fetch(cmd.Context(), repo, window, thresholds)
	if err != nil {
		return err
	}
	for _, warning := range data.Warnings {
		warn(cmd, warning)
	}
	result := health.Compute(data, window, thresholds)
	if flags.asJSON {
		return report.HealthJSON(cmd.OutOrStdout(), repo, result)
	}
	return report.HealthText(cmd.OutOrStdout(), repo, result)
}
