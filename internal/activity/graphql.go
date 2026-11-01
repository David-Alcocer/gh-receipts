package activity

import (
	"fmt"
	"slices"
)

const (
	// searchResultLimit is the most results the search connection returns.
	searchResultLimit = 1000
	// ghostLogin is how GitHub displays the author of content whose account was
	// deleted. The API returns a null author in that case.
	ghostLogin = "ghost"
)

type actor struct {
	Typename string `json:"__typename"`
	Login    string
}

func (a *actor) login() string {
	if a == nil {
		return ghostLogin
	}
	return a.Login
}

func (a *actor) isBot() bool {
	return a != nil && a.Typename == "Bot"
}

// backwardPage is a page of a connection read from its newest item.
type backwardPage[T any] struct {
	PageInfo struct {
		HasPreviousPage bool
		StartCursor     string
	}
	Nodes []T
}

// walkBack follows a connection towards older items and returns them all.
// keepGoing receives the oldest item of a page and decides whether older pages
// can still matter.
func walkBack[T any](first backwardPage[T], keepGoing func(oldest T) bool, previous func(cursor string) (backwardPage[T], error)) ([]T, error) {
	items := first.Nodes
	current := first
	for current.PageInfo.HasPreviousPage && (len(current.Nodes) == 0 || keepGoing(current.Nodes[0])) {
		page, err := previous(current.PageInfo.StartCursor)
		if err != nil {
			return nil, err
		}
		items = slices.Concat(page.Nodes, items)
		current = page
	}
	return items, nil
}

// SearchConnection is a page of search results.
type SearchConnection[T any] struct {
	IssueCount int
	PageInfo   struct {
		HasNextPage bool
		EndCursor   string
	}
	Nodes []T
}

// SearchAll reads every page of a search. It returns a warning when GitHub
// matched more results than it can return.
func SearchAll[T any](search string, next func(cursor interface{}) (SearchConnection[T], error)) ([]T, []string, error) {
	var items []T
	var cursor interface{}
	for {
		page, err := next(cursor)
		if err != nil {
			return nil, nil, err
		}
		items = slices.Concat(items, page.Nodes)
		if !page.PageInfo.HasNextPage {
			return items, truncationWarnings(search, page.IssueCount), nil
		}
		cursor = page.PageInfo.EndCursor
	}
}

func truncationWarnings(search string, issueCount int) []string {
	if issueCount <= searchResultLimit {
		return nil
	}
	return []string{fmt.Sprintf("search %q matched %d results, but GitHub only returns %d", search, issueCount, searchResultLimit)}
}
