package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

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

// criteriaWrapWidth is the most characters of an item on one line. Longer items
// continue on the next lines instead of being cut, because reading the
// criteria is the point of the command.
const criteriaWrapWidth = 64

// CriteriaText writes a summary line and every item in full. GitHub has no link
// for a single checkbox, so the first line of each item points to the issue;
// the lines that continue it don't repeat the link.
func CriteriaText(w io.Writer, issue criteria.Issue) error {
	type row struct {
		cell         string
		continuation []string
	}
	rows := []row{{cell: fmt.Sprintf("#%d %s  %s", issue.Number, clean(issue.Title), criteriaSummary(issue.Result))}}
	for _, item := range issue.Items {
		indent := "  " + strings.Repeat("  ", item.Depth)
		marker := itemMarker(item)
		lines := wrapWords(clean(item.Text), criteriaWrapWidth)
		r := row{cell: indent + marker + " " + lines[0]}
		for _, rest := range lines[1:] {
			r.continuation = append(r.continuation, indent+strings.Repeat(" ", utf8.RuneCountInString(marker)+1)+rest)
		}
		rows = append(rows, r)
	}

	width := 0
	for _, r := range rows {
		width = max(width, utf8.RuneCountInString(r.cell))
	}
	var out strings.Builder
	for _, r := range rows {
		padding := strings.Repeat(" ", width-utf8.RuneCountInString(r.cell)+2)
		fmt.Fprintf(&out, "%s%s%s\n", r.cell, padding, issue.URL)
		for _, line := range r.continuation {
			fmt.Fprintf(&out, "%s\n", line)
		}
	}
	_, err := io.WriteString(w, out.String())
	return err
}

// wrapWords splits text into lines of at most width characters, breaking only
// between words. A word longer than width, such as a link, gets a line of its
// own instead of being cut.
func wrapWords(text string, width int) []string {
	var lines []string
	current := ""
	for _, word := range strings.Fields(text) {
		switch {
		case current == "":
			current = word
		case utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	return append(lines, current)
}

// criteriaSummary counts checked boxes. Plain list items can't be checked, so
// a section of them is only counted, never shown as 0 done.
func criteriaSummary(result criteria.Result) string {
	switch {
	case !result.Found:
		return "no criteria found"
	case len(result.Items) == 0:
		return "section is empty"
	case !result.Items[0].Checkbox && len(result.Items) == 1:
		return "1 criterion, no checkboxes"
	case !result.Items[0].Checkbox:
		return fmt.Sprintf("%d criteria, no checkboxes", len(result.Items))
	}
	return fmt.Sprintf("%d/%d", result.Checked(), len(result.Items))
}

func itemMarker(item criteria.Item) string {
	switch {
	case !item.Checkbox:
		return "-"
	case item.Checked:
		return "[x]"
	}
	return "[ ]"
}
