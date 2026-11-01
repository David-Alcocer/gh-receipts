// Package project reads items from a GitHub Project.
package project

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/ghclient"
)

const (
	noStatus = "No status"
	// privateItemTitle replaces items the token can't read, which come back
	// without content.
	privateItemTitle = "(private item)"

	errorTypeMissingScope = "INSUFFICIENT_SCOPES"
	errorTypeNotFound     = "NOT_FOUND"
	requiredScope         = "read:project"
)

// ErrMissingScope means the token can't read projects.
var ErrMissingScope = errors.New("reading the project needs the read:project scope: run gh auth refresh -s read:project")

// Project is a board and its items updated since a point in time.
type Project struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Items  []Item `json:"items"`
}

// Item is a card on a board. UpdatedAt changes with any edit, so it means the
// card was updated, not that its status moved.
type Item struct {
	// Number is 0 for draft issues.
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
	// URL is the board for items with no page of their own.
	URL              string   `json:"url"`
	Assignees        []string `json:"assignees"`
	AssignedToViewer bool     `json:"assignedToViewer"`
}

type itemNode struct {
	UpdatedAt        time.Time
	FieldValueByName *struct{ Name string }
	Content          *struct {
		Number    int
		Title     string
		URL       string
		Assignees struct {
			Nodes []struct{ Login string }
		}
	}
}

type boardNode struct {
	Title string
	URL   string
	Items struct {
		PageInfo struct {
			HasNextPage bool
			EndCursor   string
		}
		Nodes []itemNode
	}
}

// Fetch returns the items of a project updated since the given time.
func Fetch(ctx context.Context, owner string, number int, since time.Time, viewer string) (Project, error) {
	client, err := ghclient.New()
	if err != nil {
		return Project{}, err
	}
	return FetchWith(ctx, client, owner, number, since, viewer)
}

// FetchWith is Fetch with a given client. It lists the viewer's items first,
// and the newest first within that.
func FetchWith(ctx context.Context, client activity.GraphQLClient, owner string, number int, since time.Time, viewer string) (Project, error) {
	// The filter only takes dates, and rejects timestamps by returning nothing.
	// Starting a day earlier covers any time zone, and the exact comparison
	// happens below.
	filter := "updated:>=" + since.UTC().AddDate(0, 0, -1).Format(time.DateOnly)

	result := Project{Number: number, Items: []Item{}}
	var cursor interface{}
	for {
		board, err := fetchPage(ctx, client, owner, number, filter, cursor)
		if err != nil {
			return Project{}, err
		}
		result.Title, result.URL = board.Title, board.URL
		for _, node := range board.Items.Nodes {
			if !node.UpdatedAt.Before(since) {
				result.Items = append(result.Items, newItem(node, board.URL, viewer))
			}
		}
		if !board.Items.PageInfo.HasNextPage {
			break
		}
		cursor = board.Items.PageInfo.EndCursor
	}

	slices.SortStableFunc(result.Items, compareItems)
	return result, nil
}

func fetchPage(ctx context.Context, client activity.GraphQLClient, owner string, number int, filter string, cursor interface{}) (boardNode, error) {
	var response struct {
		RepositoryOwner *struct{ ProjectV2 *boardNode }
	}
	variables := map[string]interface{}{"owner": owner, "number": number, "filter": filter, "cursor": cursor}
	if err := client.DoWithContext(ctx, projectQuery, variables, &response); err != nil {
		return boardNode{}, translateError(err, owner, number)
	}
	if response.RepositoryOwner == nil || response.RepositoryOwner.ProjectV2 == nil {
		return boardNode{}, notFoundError(owner, number)
	}
	return *response.RepositoryOwner.ProjectV2, nil
}

func translateError(err error, owner string, number int) error {
	var graphQLError *api.GraphQLError
	if errors.As(err, &graphQLError) {
		for _, item := range graphQLError.Errors {
			if item.Type == errorTypeMissingScope || strings.Contains(item.Message, requiredScope) {
				return ErrMissingScope
			}
			if item.Type == errorTypeNotFound {
				return notFoundError(owner, number)
			}
		}
	}
	return fmt.Errorf("read project %d of %s: %w", number, owner, err)
}

func notFoundError(owner string, number int) error {
	return fmt.Errorf("project %d of %s not found: check the number in --project or .github/receipts.yml", number, owner)
}

func newItem(node itemNode, boardURL, viewer string) Item {
	item := Item{
		Title:     privateItemTitle,
		Status:    noStatus,
		UpdatedAt: node.UpdatedAt,
		URL:       boardURL,
		Assignees: []string{},
	}
	if node.FieldValueByName != nil && node.FieldValueByName.Name != "" {
		item.Status = node.FieldValueByName.Name
	}
	if node.Content == nil {
		return item
	}

	item.Number = node.Content.Number
	item.Title = node.Content.Title
	if node.Content.URL != "" {
		item.URL = node.Content.URL
	}
	for _, assignee := range node.Content.Assignees.Nodes {
		item.Assignees = append(item.Assignees, assignee.Login)
	}
	item.AssignedToViewer = slices.Contains(item.Assignees, viewer)
	return item
}

func compareItems(a, b Item) int {
	if a.AssignedToViewer != b.AssignedToViewer {
		if a.AssignedToViewer {
			return -1
		}
		return 1
	}
	return cmp.Compare(b.UpdatedAt.UnixNano(), a.UpdatedAt.UnixNano())
}
