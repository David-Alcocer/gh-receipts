// Package config reads a repository's .github/receipts.yml, which every command
// shares.
package config

import (
	"context"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
	"gopkg.in/yaml.v3"

	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

// configQuery reads the configuration from the default branch, so it also works
// with --repo outside a clone.
const configQuery = `query ReceiptsConfig($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    object(expression: "HEAD:.github/receipts.yml") {
      ... on Blob { text }
    }
  }
}
`

// Config is the repository's configuration. Zero values mean "not set".
type Config struct {
	Project int
	Health  Health
}

// Health holds the thresholds of the health command. A zero field means the
// command's default applies.
type Health struct {
	MaxPRSize           int
	MaxFirstReviewHours int
	MaxMergeHours       int
	StaleDays           int
}

// file is the YAML layout. Pointers tell an explicit 0, which is invalid, from
// a missing value.
type file struct {
	Project int `yaml:"project"`
	Health  struct {
		MaxPRSize           *int `yaml:"maxPRSize"`
		MaxFirstReviewHours *int `yaml:"maxFirstReviewHours"`
		MaxMergeHours       *int `yaml:"maxMergeHours"`
		StaleDays           *int `yaml:"staleDays"`
	} `yaml:"health"`
}

// GraphQLClient is the part of the go-gh client this package uses.
type GraphQLClient interface {
	DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error
}

// Read returns the configuration of repo, or a zero Config when it has no file.
func Read(ctx context.Context, repo repository.Repository) (Config, error) {
	client, err := ghclient.New()
	if err != nil {
		return Config{}, err
	}
	return ReadWith(ctx, client, repo)
}

// ReadWith is Read with a given client.
func ReadWith(ctx context.Context, client GraphQLClient, repo repository.Repository) (Config, error) {
	var response struct {
		Repository struct {
			Object *struct{ Text string }
		}
	}
	variables := map[string]interface{}{"owner": repo.Owner, "name": repo.Name}
	if err := client.DoWithContext(ctx, configQuery, variables, &response); err != nil {
		return Config{}, fmt.Errorf("read .github/receipts.yml: %w", err)
	}
	if response.Repository.Object == nil {
		return Config{}, nil
	}

	var parsed file
	if err := yaml.Unmarshal([]byte(response.Repository.Object.Text), &parsed); err != nil {
		return Config{}, fmt.Errorf("invalid .github/receipts.yml: %w", err)
	}
	return validate(parsed)
}

func validate(parsed file) (Config, error) {
	if parsed.Project < 0 {
		return Config{}, fmt.Errorf("invalid .github/receipts.yml: project must be a positive number, got %d", parsed.Project)
	}
	thresholds := []struct {
		name  string
		value *int
	}{
		{"maxPRSize", parsed.Health.MaxPRSize},
		{"maxFirstReviewHours", parsed.Health.MaxFirstReviewHours},
		{"maxMergeHours", parsed.Health.MaxMergeHours},
		{"staleDays", parsed.Health.StaleDays},
	}
	for _, threshold := range thresholds {
		if threshold.value != nil && *threshold.value <= 0 {
			return Config{}, fmt.Errorf("invalid .github/receipts.yml: health.%s must be a positive number, got %d", threshold.name, *threshold.value)
		}
	}

	return Config{
		Project: parsed.Project,
		Health: Health{
			MaxPRSize:           valueOrZero(parsed.Health.MaxPRSize),
			MaxFirstReviewHours: valueOrZero(parsed.Health.MaxFirstReviewHours),
			MaxMergeHours:       valueOrZero(parsed.Health.MaxMergeHours),
			StaleDays:           valueOrZero(parsed.Health.StaleDays),
		},
	}, nil
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
