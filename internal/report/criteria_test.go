package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/David-Alcocer/gh-receipts/internal/criteria"
)

const testIssueURL = "https://github.com/o/r/issues/5"

func testIssue(result criteria.Result) criteria.Issue {
	return criteria.Issue{Number: 5, Title: "Parser", State: "OPEN", URL: testIssueURL, Result: result}
}

func renderCriteria(t *testing.T, issue criteria.Issue) string {
	t.Helper()
	var out bytes.Buffer
	if err := CriteriaText(&out, issue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out.String()
}

func TestCriteriaTextShowsEachItemWithItsSource(t *testing.T) {
	got := renderCriteria(t, testIssue(criteria.Result{Found: true, Items: []criteria.Item{
		{Text: "First", Checkbox: true, Checked: true},
		{Text: "Nested", Checkbox: true, Depth: 1},
		{Text: "Second", Checkbox: true},
	}}))

	want := "" +
		"#5 Parser  1/3  " + testIssueURL + "\n" +
		"  [x] First     " + testIssueURL + "\n" +
		"    [ ] Nested  " + testIssueURL + "\n" +
		"  [ ] Second    " + testIssueURL + "\n"
	if got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestCriteriaTextDoesNotShowPlainItemsAsUnchecked(t *testing.T) {
	got := renderCriteria(t, testIssue(criteria.Result{Found: true, Items: []criteria.Item{
		{Text: "First"},
		{Text: "Nested", Depth: 1},
	}}))

	want := "" +
		"#5 Parser  2 criteria, no checkboxes  " + testIssueURL + "\n" +
		"  - First                             " + testIssueURL + "\n" +
		"    - Nested                          " + testIssueURL + "\n"
	if got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestCriteriaTextExplainsMissingAndEmptySections(t *testing.T) {
	tests := []struct {
		name   string
		result criteria.Result
		want   string
	}{
		{"missing", criteria.Result{Items: []criteria.Item{}}, "#5 Parser  no criteria found  " + testIssueURL + "\n"},
		{"empty", criteria.Result{Found: true, Items: []criteria.Item{}}, "#5 Parser  section is empty  " + testIssueURL + "\n"},
		{"one plain item", criteria.Result{Found: true, Items: []criteria.Item{{Text: "Only"}}}, "#5 Parser  1 criterion, no checkboxes  " + testIssueURL + "\n  - Only                               " + testIssueURL + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderCriteria(t, testIssue(tt.result)); got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCriteriaTextRemovesControlCharacters(t *testing.T) {
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: "Safe\x1b[2J", Checkbox: true}}})
	issue.Title = "Title\x1b]0;x\a"

	got := renderCriteria(t, issue)

	if strings.ContainsAny(got, "\x1b\a") {
		t.Errorf("text = %q", got)
	}
}

func TestCriteriaTextShowsLongItemsInFull(t *testing.T) {
	long := "The text output of criteria shows every item in full, without cutting it with an ellipsis"
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{
		{Text: long, Checkbox: true},
		{Text: "Short", Checkbox: true, Depth: 1},
	}})

	want := "" +
		"#5 Parser  0/2                                                       " + testIssueURL + "\n" +
		"  [ ] The text output of criteria shows every item in full, without  " + testIssueURL + "\n" +
		"      cutting it with an ellipsis\n" +
		"    [ ] Short                                                        " + testIssueURL + "\n"
	if got := renderCriteria(t, issue); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestCriteriaTextIndentsContinuationsOfNestedItems(t *testing.T) {
	long := strings.Repeat("word ", 20)
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: long, Depth: 2}}})

	lines := strings.Split(strings.TrimSuffix(renderCriteria(t, issue), "\n"), "\n")

	if len(lines) != 3 || !strings.HasPrefix(lines[1], "      - word") || !strings.HasPrefix(lines[2], "        word") || strings.Contains(lines[2], "https://") {
		t.Errorf("lines = %q", lines)
	}
}

func TestCriteriaTextKeepsLongWordsWhole(t *testing.T) {
	link := "https://example.com/" + strings.Repeat("x", criteriaWrapWidth)
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: "See " + link + " first", Checkbox: true}}})

	got := renderCriteria(t, issue)

	if !strings.Contains(got, "\n      "+link+"\n") || !strings.Contains(got, "[ ] See  ") || !strings.Contains(got, "\n      first\n") {
		t.Errorf("text = %q", got)
	}
}

func TestCriteriaTextHasNoTrailingSpaces(t *testing.T) {
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: strings.Repeat("word ", 30), Checkbox: true}}})

	for _, line := range strings.Split(renderCriteria(t, issue), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("trailing spaces in %q", line)
		}
	}
}

func TestCriteriaJSONKeepsTheTextAsWritten(t *testing.T) {
	long := strings.Repeat("a", criteriaWrapWidth+1) + " end"
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: long}}})
	var out bytes.Buffer

	if err := CriteriaJSON(&out, testRepo, issue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), long) {
		t.Errorf("JSON = %s", out.String())
	}
}

func TestCriteriaJSON(t *testing.T) {
	var out bytes.Buffer
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: "Use <b>", Checkbox: true, Checked: true, Depth: 1}}})

	if err := CriteriaJSON(&out, testRepo, issue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{
  "schemaVersion": 2,
  "repository": "o/r",
  "number": 5,
  "title": "Parser",
  "state": "OPEN",
  "url": "https://github.com/o/r/issues/5",
  "found": true,
  "items": [
    {
      "text": "Use <b>",
      "checked": true,
      "depth": 1,
      "checkbox": true
    }
  ]
}
`
	if got := out.String(); got != want {
		t.Errorf("JSON =\n%s\nwant\n%s", got, want)
	}
}

func TestCriteriaJSONHasNoCheckedValueForPlainItems(t *testing.T) {
	var out bytes.Buffer
	issue := testIssue(criteria.Parse("## Criterios de aceptación\n- Primero\n"))

	if err := CriteriaJSON(&out, testRepo, issue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := `"checked": null,`; !strings.Contains(out.String(), want) {
		t.Errorf("JSON =\n%s\nwant to contain %s", out.String(), want)
	}
}

func TestCriteriaJSONUsesAnEmptyList(t *testing.T) {
	var out bytes.Buffer

	if err := CriteriaJSON(&out, testRepo, testIssue(criteria.Result{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document struct {
		Found bool
		Items json.RawMessage
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Found || string(document.Items) != "[]" {
		t.Errorf("document = %+v, items = %s", document, document.Items)
	}
}

func TestWrapWordsFillsLinesUpToTheWidth(t *testing.T) {
	exact := strings.Repeat("a", 30) + " " + strings.Repeat("b", 33)

	if got := wrapWords(exact+" c", len(exact)); len(got) != 2 || got[0] != exact || got[1] != "c" {
		t.Errorf("lines = %q", got)
	}
}
