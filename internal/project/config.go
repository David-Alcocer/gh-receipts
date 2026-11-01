package project

import (
	"context"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
	"gopkg.in/yaml.v3"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

type config struct {
	Project int `yaml:"project"`
}

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
	var response struct {
		Repository struct {
			Object *struct{ Text string }
		}
	}
	variables := map[string]interface{}{"owner": repo.Owner, "name": repo.Name}
	if err := client.DoWithContext(ctx, configQuery, variables, &response); err != nil {
		return 0, fmt.Errorf("read .github/receipts.yml: %w", err)
	}
	if response.Repository.Object == nil {
		return 0, nil
	}

	var parsed config
	if err := yaml.Unmarshal([]byte(response.Repository.Object.Text), &parsed); err != nil {
		return 0, fmt.Errorf("invalid .github/receipts.yml: %w", err)
	}
	if parsed.Project < 0 {
		return 0, fmt.Errorf("invalid .github/receipts.yml: project must be a positive number, got %d", parsed.Project)
	}
	return parsed.Project, nil
}
