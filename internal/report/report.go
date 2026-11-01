// Package report renders activity as text for people and JSON for agents. It
// never fetches data, so other interfaces can reuse the same activity.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/project"
)

// schemaVersion changes when the JSON output changes in an incompatible way.
// docs/today-json.md documents the schema.
const schemaVersion = 1

// Info describes the report around the activity.
type Info struct {
	Repo  repository.Repository
	Since time.Time
	// SinceSource explains where Since came from, such as "last session".
	SinceSource string
	// Project is nil when no project is selected.
	Project *project.Project
}

// document is the JSON output.
type document struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Repository     string                 `json:"repository"`
	Since          time.Time              `json:"since"`
	Viewer         string                 `json:"viewer"`
	DefaultBranch  string                 `json:"defaultBranch"`
	MergedPRs      []activity.PullRequest `json:"mergedPRs"`
	OpenedPRs      []activity.PullRequest `json:"openedPRs"`
	ReviewRequests []activity.PullRequest `json:"reviewRequests"`
	Reviews        []activity.Review      `json:"reviews"`
	Comments       []activity.Comment     `json:"comments"`
	FailedChecks   []activity.FailedCheck `json:"failedChecks"`
	Project        *project.Project       `json:"project"`
	Warnings       []string               `json:"warnings"`
}

// JSON writes the complete activity. Unlike Text, it never limits sections.
func JSON(w io.Writer, a activity.Activity, info Info) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(document{
		SchemaVersion:  schemaVersion,
		Repository:     info.Repo.Owner + "/" + info.Repo.Name,
		Since:          info.Since,
		Viewer:         a.Viewer,
		DefaultBranch:  a.DefaultBranch,
		MergedPRs:      nonNil(a.MergedPRs),
		OpenedPRs:      nonNil(a.OpenedPRs),
		ReviewRequests: nonNil(a.ReviewRequests),
		Reviews:        nonNil(a.Reviews),
		Comments:       nonNil(a.Comments),
		FailedChecks:   nonNil(a.FailedChecks),
		Project:        info.Project,
		Warnings:       nonNil(a.Warnings),
	})
	if err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

// nonNil makes an empty list encode as [] instead of null.
func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
