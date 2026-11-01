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
		{Text: "First", Checked: true},
		{Text: "Nested", Depth: 1},
		{Text: "Second"},
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

func TestCriteriaTextExplainsMissingAndEmptySections(t *testing.T) {
	tests := []struct {
		name   string
		result criteria.Result
		want   string
	}{
		{"missing", criteria.Result{Items: []criteria.Item{}}, "#5 Parser  no criteria found  " + testIssueURL + "\n"},
		{"empty", criteria.Result{Found: true, Items: []criteria.Item{}}, "#5 Parser  section is empty  " + testIssueURL + "\n"},
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
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: "Safe\x1b[2J"}}})
	issue.Title = "Title\x1b]0;x\a"

	got := renderCriteria(t, issue)

	if strings.ContainsAny(got, "\x1b\a") {
		t.Errorf("text = %q", got)
	}
}

func TestCriteriaTextShortensLongItemsButJSONDoesNot(t *testing.T) {
	long := strings.Repeat("a", lineExcerptLength+1)
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: long}}})

	if got := renderCriteria(t, issue); strings.Contains(got, long) || !strings.Contains(got, strings.Repeat("a", lineExcerptLength)+"…") {
		t.Errorf("text = %q", got)
	}

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
	issue := testIssue(criteria.Result{Found: true, Items: []criteria.Item{{Text: "Use <b>", Checked: true, Depth: 1}}})

	if err := CriteriaJSON(&out, testRepo, issue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{
  "schemaVersion": 1,
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
      "depth": 1
    }
  ]
}
`
	if got := out.String(); got != want {
		t.Errorf("JSON =\n%s\nwant\n%s", got, want)
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
