package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/criteria"
)

// criteriaSchemaVersion changes when the JSON output changes in an incompatible
// way. docs/criteria-json.md documents the schema.
const criteriaSchemaVersion = 1

type criteriaDocument struct {
	SchemaVersion int             `json:"schemaVersion"`
	Repository    string          `json:"repository"`
	Number        int             `json:"number"`
	Title         string          `json:"title"`
	State         string          `json:"state"`
	URL           string          `json:"url"`
	Found         bool            `json:"found"`
	Items         []criteria.Item `json:"items"`
}

// CriteriaJSON writes an issue's parsed criteria.
func CriteriaJSON(w io.Writer, repo repository.Repository, issue criteria.Issue) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(criteriaDocument{
		SchemaVersion: criteriaSchemaVersion,
		Repository:    repo.Owner + "/" + repo.Name,
		Number:        issue.Number,
		Title:         issue.Title,
		State:         issue.State,
		URL:           issue.URL,
		Found:         issue.Found,
		Items:         nonNil(issue.Items),
	})
	if err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

// CriteriaText writes a summary line and every item, shortened to fit a line.
// GitHub has no link for a single checkbox, so each line points to the issue.
func CriteriaText(w io.Writer, issue criteria.Issue) error {
	columns := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(columns, "#%d %s  %s\t%s\n", issue.Number, clean(issue.Title), criteriaSummary(issue.Result), issue.URL)
	for _, item := range issue.Items {
		fmt.Fprintf(columns, "  %s[%s] %s\t%s\n", strings.Repeat("  ", item.Depth), checkMark(item), shorten(clean(item.Text)), issue.URL)
	}
	return columns.Flush()
}

func criteriaSummary(result criteria.Result) string {
	switch {
	case !result.Found:
		return "no criteria found"
	case len(result.Items) == 0:
		return "section is empty"
	}
	return fmt.Sprintf("%d/%d", result.Checked(), len(result.Items))
}

func checkMark(item criteria.Item) string {
	if item.Checked {
		return "x"
	}
	return " "
}
