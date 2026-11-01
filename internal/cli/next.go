package cli

import (
	"context"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/David-Alcocer/gh-receipts/internal/pending"
	"github.com/David-Alcocer/gh-receipts/internal/report"
)

// nextDeps holds everything the command needs from the outside world. It has no
// session store: next doesn't read or change the last session of today.
type nextDeps struct {
	currentRepo func() (repository.Repository, error)
	fetch       func(ctx context.Context, repo repository.Repository) (pending.Pending, error)
}

type nextFlags struct {
	repo   string
	asJSON bool
}

func defaultNextDeps() nextDeps {
	return nextDeps{
		currentRepo: repository.Current,
		fetch:       pending.Fetch,
	}
}

func newNextCmd(deps nextDeps) *cobra.Command {
	var flags nextFlags
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show what's left to do",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNext(cmd, deps, flags)
		},
	}
	cmd.Flags().StringVar(&flags.repo, "repo", "", "repository to report on, as owner/name")
	cmd.Flags().BoolVar(&flags.asJSON, "json", false, "print the complete data as JSON")
	return cmd
}

func runNext(cmd *cobra.Command, deps nextDeps, flags nextFlags) error {
	repo, err := resolveRepo(deps.currentRepo, flags.repo)
	if err != nil {
		return err
	}

	result, err := deps.fetch(cmd.Context(), repo)
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		warn(cmd, warning)
	}
	if flags.asJSON {
		return report.NextJSON(cmd.OutOrStdout(), repo, result)
	}
	return report.NextText(cmd.OutOrStdout(), repo, result)
}
