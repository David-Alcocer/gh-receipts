// Package ghclient creates the GitHub API clients used by gh-receipts.
package ghclient

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
)

// New returns a GraphQL client that uses the host and credentials configured
// in the GitHub CLI.
func New() (*api.GraphQLClient, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("create GitHub client: %w", err)
	}
	return client, nil
}
