package criteria

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Result
	}{
		{
			name: "issue form output",
			body: "### Context\n\nWhy.\n\n### Acceptance criteria\n\n- [ ] First\n- [x] Second\n\n### Notes and hints\n\n_No response_\n\n### Out of scope\n\n_No response_\n",
			want: Result{Found: true, Items: []Item{{Text: "First"}, {Text: "Second", Checked: true}}},
		},
		{
			name: "hand-written markdown",
			body: "Intro\n\n## acceptance CRITERIA\n\nSome text.\n\n* [X] Upper case mark\n+ [ ] Plus marker\n1. [ ] Numbered\n2) [x] Other number style\n- Not a checkbox\n",
			want: Result{Found: true, Items: []Item{
				{Text: "Upper case mark", Checked: true},
				{Text: "Plus marker"},
				{Text: "Numbered"},
				{Text: "Other number style", Checked: true},
			}},
		},
		{
			name: "nested lists",
			body: "## Acceptance criteria\n- [ ] Parent\n  - [x] Child\n    - [ ] Grandchild\n  - [ ] Second child\n- [ ] Next parent\n\t- [ ] Tab child\n",
			want: Result{Found: true, Items: []Item{
				{Text: "Parent"},
				{Text: "Child", Checked: true, Depth: 1},
				{Text: "Grandchild", Depth: 2},
				{Text: "Second child", Depth: 1},
				{Text: "Next parent"},
				{Text: "Tab child", Depth: 1},
			}},
		},
		{
			name: "missing section",
			body: "### Context\n\n- [ ] Not a criterion\n",
			want: Result{Items: []Item{}},
		},
		{
			name: "empty body",
			body: "",
			want: Result{Items: []Item{}},
		},
		{
			name: "empty section",
			body: "### Acceptance criteria\n\n### Notes\n\n- [ ] Not a criterion\n",
			want: Result{Found: true, Items: []Item{}},
		},
		{
			name: "template placeholder is not a criterion",
			body: "### Acceptance criteria\n\n- [ ]\n- [ ]   \n",
			want: Result{Found: true, Items: []Item{}},
		},
		{
			name: "section followed by another heading",
			body: "## Acceptance criteria\n- [ ] Inside\n## Next\n- [ ] Outside\n",
			want: Result{Found: true, Items: []Item{{Text: "Inside"}}},
		},
		{
			name: "higher heading ends the section",
			body: "### Acceptance criteria\n- [ ] Inside\n# Top\n- [ ] Outside\n",
			want: Result{Found: true, Items: []Item{{Text: "Inside"}}},
		},
		{
			name: "deeper heading stays in the section",
			body: "### Acceptance criteria\n- [ ] Before\n#### Group\n- [ ] After\n### Next\n- [ ] Outside\n",
			want: Result{Found: true, Items: []Item{{Text: "Before"}, {Text: "After"}}},
		},
		{
			name: "code block is only an example",
			body: "## Acceptance criteria\n- [ ] Real\n```md\n- [ ] Example\n## Not a heading\n```\n~~~~\n```\n- [ ] Other example\n~~~~\n- [ ] Also real\n",
			want: Result{Found: true, Items: []Item{{Text: "Real"}, {Text: "Also real"}}},
		},
		{
			name: "other fence character doesn't close a code block",
			body: "## Acceptance criteria\n~~~\n```\n- [ ] Example\n~~~\n- [ ] Real\n",
			want: Result{Found: true, Items: []Item{{Text: "Real"}}},
		},
		{
			name: "shorter fence doesn't close a code block",
			body: "## Acceptance criteria\n````\n```\n- [ ] Example\n````\n- [ ] Real\n",
			want: Result{Found: true, Items: []Item{{Text: "Real"}}},
		},
		{
			name: "fence with text after it doesn't close a code block",
			body: "## Acceptance criteria\n```\n``` md\n- [ ] Example\n```\n- [ ] Real\n",
			want: Result{Found: true, Items: []Item{{Text: "Real"}}},
		},
		{
			name: "same name nested doesn't change where the section ends",
			body: "## Acceptance criteria\n### Acceptance criteria\n### Other\n- [ ] Still inside\n## Next\n- [ ] Outside\n",
			want: Result{Found: true, Items: []Item{{Text: "Still inside"}}},
		},
		{
			name: "windows line endings after closing hashes",
			body: "## Acceptance criteria ##\r\n- [ ] First\r\n",
			want: Result{Found: true, Items: []Item{{Text: "First"}}},
		},
		{
			name: "heading inside a code block is not the section",
			body: "```\n## Acceptance criteria\n- [ ] Example\n```\n",
			want: Result{Items: []Item{}},
		},
		{
			name: "first section wins",
			body: "## Acceptance criteria\n- [ ] First\n## Acceptance criteria\n- [ ] Second\n",
			want: Result{Found: true, Items: []Item{{Text: "First"}}},
		},
		{
			name: "windows line endings",
			body: "## Acceptance criteria\r\n\r\n- [ ] First\r\n- [x] Second\r\n",
			want: Result{Found: true, Items: []Item{{Text: "First"}, {Text: "Second", Checked: true}}},
		},
		{
			name: "closing hashes and indented heading",
			body: "  ## Acceptance criteria ##\n- [ ] First\n",
			want: Result{Found: true, Items: []Item{{Text: "First"}}},
		},
		{
			name: "other heading levels and names",
			body: "# Acceptance criteria\n- [ ] One\n#### Acceptance criteria\n- [ ] Two\n## Acceptance criteria extra\n- [ ] Three\n#Acceptance criteria\n",
			want: Result{Items: []Item{}},
		},
		{
			name: "marker needs a space before the text",
			body: "## Acceptance criteria\n- [ ]No space\n- [y] Other mark\n-[ ] No list space\n",
			want: Result{Found: true, Items: []Item{}},
		},
		{
			name: "only the first line of an item",
			body: "## Acceptance criteria\n- [ ] First line\n  second line\n",
			want: Result{Found: true, Items: []Item{{Text: "First line"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestChecked(t *testing.T) {
	result := Parse("## Acceptance criteria\n- [x] One\n- [ ] Two\n  - [X] Three\n")

	if got := result.Checked(); got != 2 {
		t.Errorf("Checked() = %d, want 2", got)
	}
}
