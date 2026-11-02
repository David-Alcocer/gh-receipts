// Package criteria reads the acceptance criteria checklist of an issue.
package criteria

import (
	"regexp"
	"strings"
)

// tabWidth is how many columns a tab indents a list item.
const tabWidth = 4

// sectionNames are the headings that start the criteria section, compared
// without case.
var sectionNames = []string{"acceptance criteria", "criterios de aceptación", "criterios de aceptacion"}

var (
	headingPattern  = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))??(?:[ \t]+#+)?[ \t]*$`)
	checkboxPattern = regexp.MustCompile(`^([ \t]*)(?:[-*+]|\d{1,9}[.)])[ \t]+\[([ xX])\](?:[ \t]+(.*))?$`)
	listItemPattern = regexp.MustCompile(`^([ \t]*)(?:[-*+]|\d{1,9}[.)])[ \t]+(.*)$`)
	// A malformed box like "[y]" or "[ ]text" is neither a checkbox nor plain text.
	malformedBoxPattern = regexp.MustCompile(`^\[.\]`)
	breakPattern        = regexp.MustCompile(`^ {0,3}(?:(?:-[ \t]*){3,}|(?:\*[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	fencePattern        = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})")
)

// Item is one criterion. Depth is 0 for top-level items, and each nested list
// adds one. Checkbox is false for a plain list item, which can't be checked.
type Item struct {
	Text     string `json:"text"`
	Checked  bool   `json:"checked"`
	Depth    int    `json:"depth"`
	Checkbox bool   `json:"checkbox"`
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

// Parse reads the first "Acceptance criteria" or "Criterios de aceptación"
// section of an issue body, with a ## or ### heading. The criteria are its
// checkboxes or, in a section without any, its plain list items. Bodies are
// written by people, so anything else is ignored.
func Parse(body string) Result {
	found := false
	sectionLevel := 0
	fence := ""
	var boxes, plain list

lines:
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
			if found && level <= sectionLevel {
				break lines
			}
			if !found && isSectionHeading(level, heading[2]) {
				found = true
				sectionLevel = level
			}
			continue
		}

		if !found {
			continue
		}
		if box := checkboxPattern.FindStringSubmatch(text); box != nil {
			// The form template starts with an empty "- [ ]", which isn't a criterion.
			if label := strings.TrimSpace(box[3]); label != "" {
				boxes.add(box[1], Item{Text: label, Checked: box[2] != " ", Checkbox: true})
			}
			continue
		}
		if item := listItemPattern.FindStringSubmatch(text); item != nil && !breakPattern.MatchString(text) {
			label := strings.TrimSpace(item[2])
			if label != "" && !malformedBoxPattern.MatchString(label) {
				plain.add(item[1], Item{Text: label})
			}
		}
	}

	// A checklist with notes under its items keeps only the checkboxes.
	items := boxes.items
	if len(items) == 0 {
		items = plain.items
	}
	if items == nil {
		items = []Item{}
	}
	return Result{Found: found, Items: items}
}

// list collects items of one kind with their nesting depth.
type list struct {
	items   []Item
	indents []int
}

func (l *list) add(indent string, item Item) {
	width := indentWidth(indent)
	l.indents = dropIndentsFrom(l.indents, width)
	item.Depth = len(l.indents)
	l.items = append(l.items, item)
	l.indents = append(l.indents, width)
}

func isSectionHeading(level int, name string) bool {
	if level != 2 && level != 3 {
		return false
	}
	name = strings.TrimSpace(name)
	for _, sectionName := range sectionNames {
		if strings.EqualFold(name, sectionName) {
			return true
		}
	}
	return false
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
