// Package criteria reads the acceptance criteria checklist of an issue.
package criteria

import (
	"regexp"
	"strings"
)

const (
	sectionName = "acceptance criteria"
	// tabWidth is how many columns a tab indents a list item.
	tabWidth = 4
)

var (
	headingPattern  = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))??(?:[ \t]+#+)?[ \t]*$`)
	checkboxPattern = regexp.MustCompile(`^([ \t]*)(?:[-*+]|\d{1,9}[.)])[ \t]+\[([ xX])\](?:[ \t]+(.*))?$`)
	fencePattern    = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})")
)

// Item is one checklist item. Depth is 0 for top-level items, and each nested
// checklist adds one.
type Item struct {
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
	Depth   int    `json:"depth"`
}

// Result is what an issue body says about its criteria. Found is false when
// there is no section, which is an answer and not an error.
type Result struct {
	Found bool   `json:"found"`
	Items []Item `json:"items"`
}

// Checked counts the checked items.
func (r Result) Checked() int {
	checked := 0
	for _, item := range r.Items {
		if item.Checked {
			checked++
		}
	}
	return checked
}

// Parse reads the first "Acceptance criteria" section of an issue body, with a
// ## or ### heading. Bodies are written by people, so anything else is ignored.
func Parse(body string) Result {
	result := Result{Items: []Item{}}
	sectionLevel := 0
	fence := ""
	var indents []int

	for _, text := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if fence != "" {
			if closesFence(text, fence) {
				fence = ""
			}
			continue
		}
		if match := fencePattern.FindStringSubmatch(text); match != nil {
			fence = match[1]
			continue
		}

		if heading := headingPattern.FindStringSubmatch(text); heading != nil {
			level := len(heading[1])
			if result.Found && level <= sectionLevel {
				return result
			}
			if !result.Found && isSectionHeading(level, heading[2]) {
				result.Found = true
				sectionLevel = level
			}
			continue
		}

		if !result.Found {
			continue
		}
		if box := checkboxPattern.FindStringSubmatch(text); box != nil {
			// The form template starts with an empty "- [ ]", which isn't a criterion.
			label := strings.TrimSpace(box[3])
			if label == "" {
				continue
			}
			indent := indentWidth(box[1])
			indents = dropIndentsFrom(indents, indent)
			result.Items = append(result.Items, Item{Text: label, Checked: box[2] != " ", Depth: len(indents)})
			indents = append(indents, indent)
		}
	}
	return result
}

func isSectionHeading(level int, name string) bool {
	return (level == 2 || level == 3) && strings.EqualFold(strings.TrimSpace(name), sectionName)
}

// closesFence follows Markdown: a code block ends with the same character,
// repeated at least as many times.
func closesFence(text, fence string) bool {
	match := fencePattern.FindStringSubmatch(text)
	if match == nil || match[1][0] != fence[0] || len(match[1]) < len(fence) {
		return false
	}
	return strings.TrimSpace(strings.TrimLeft(text, " \t`~")) == ""
}

func indentWidth(indent string) int {
	return len(strings.ReplaceAll(indent, "\t", strings.Repeat(" ", tabWidth)))
}

// dropIndentsFrom keeps only the indents of the parents of an item at indent.
func dropIndentsFrom(indents []int, indent int) []int {
	for len(indents) > 0 && indents[len(indents)-1] >= indent {
		indents = indents[:len(indents)-1]
	}
	return indents
}
