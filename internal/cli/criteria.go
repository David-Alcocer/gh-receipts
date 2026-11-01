package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/David-Alcocer/gh-receipts/internal/criteria"
	"github.com/David-Alcocer/gh-receipts/internal/report"
)

// criteriaDeps holds everything the command needs from the outside world.
type criteriaDeps struct {
	currentRepo func() (repository.Repository, error)
	fetch       func(ctx context.Context, repo repository.Repository, number int) (criteria.Issue, error)
}

type criteriaFlags struct {
	repo   string
	asJSON bool
}

func defaultCriteriaDeps() criteriaDeps {
	return criteriaDeps{
		currentRepo: repository.Current,
		fetch:       criteria.Fetch,
	}
}

func newCriteriaCmd(deps criteriaDeps) *cobra.Command {
	var flags criteriaFlags
	cmd := &cobra.Command{
		Use:   "criteria <issue>",
		Short: "Show an issue's acceptance criteria checklist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCriteria(cmd, deps, flags, args[0])
		},
	}
	cmd.Flags().StringVar(&flags.repo, "repo", "", "repository of the issue, as owner/name")
	cmd.Flags().BoolVar(&flags.asJSON, "json", false, "print the parsed criteria as JSON")
	return cmd
}

func runCriteria(cmd *cobra.Command, deps criteriaDeps, flags criteriaFlags, arg string) error {
	number, err := strconv.Atoi(arg)
	if err != nil || number <= 0 {
		return fmt.Errorf("invalid issue %q: it must be a positive number", arg)
	}
	repo, err := resolveRepo(deps.currentRepo, flags.repo)
	if err != nil {
		return err
	}

	issue, err := deps.fetch(cmd.Context(), repo, number)
	if err != nil {
		return err
	}
	if flags.asJSON {
		return report.CriteriaJSON(cmd.OutOrStdout(), repo, issue)
	}
	return report.CriteriaText(cmd.OutOrStdout(), issue)
}
