package project

import (
	"context"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/config"
	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

// ConfiguredNumber returns the project number in the repository's
// .github/receipts.yml, or 0 when there is none.
func ConfiguredNumber(ctx context.Context, repo repository.Repository) (int, error) {
	client, err := ghclient.New()
	if err != nil {
		return 0, err
	}
	return ConfiguredNumberWith(ctx, client, repo)
}

// ConfiguredNumberWith is ConfiguredNumber with a given client.
func ConfiguredNumberWith(ctx context.Context, client activity.GraphQLClient, repo repository.Repository) (int, error) {
	parsed, err := config.ReadWith(ctx, client, repo)
	if err != nil {
		return 0, err
	}
	return parsed.Project, nil
}
