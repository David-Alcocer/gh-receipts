package project

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

var testRepo = repository.Repository{Host: "github.com", Owner: "octocat", Name: "r"}

type fakeClient struct {
	t         *testing.T
	fixtures  map[string]string
	variables []map[string]interface{}
	err       error
}

func (c *fakeClient) DoWithContext(_ context.Context, query string, variables map[string]interface{}, response interface{}) error {
	c.t.Helper()
	c.variables = append(c.variables, variables)
	if c.err != nil {
		return c.err
	}
	cursor, _ := variables["cursor"].(string)
	name, ok := c.fixtures[cursor]
	if !ok {
		c.t.Fatalf("unexpected request with cursor %q", cursor)
	}
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		c.t.Fatal(err)
	}
	var envelope struct{ Data json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		c.t.Fatal(err)
	}
	return json.Unmarshal(envelope.Data, response)
}

func date(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func syntheticClient(t *testing.T) *fakeClient {
	t.Helper()
	return &fakeClient{t: t, fixtures: map[string]string{"": "synthetic_page1.json", "P1": "synthetic_page2.json"}}
}

func fetchSynthetic(t *testing.T) (Project, *fakeClient) {
	t.Helper()
	client := syntheticClient(t)
	got, err := FetchWith(context.Background(), client, "octocat", 3, date(t, "2026-10-01T00:00:00Z"), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return got, client
}

func TestFetchRecordedOrganizationProject(t *testing.T) {
	client := &fakeClient{t: t, fixtures: map[string]string{"": "org_project.json"}}

	got, err := FetchWith(context.Background(), client, "github", 4247, date(t, "2026-09-10T00:00:00Z"), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Number != 4247 || got.Title != "GitHub Public Roadmap" || got.URL != "https://github.com/orgs/github/projects/4247" {
		t.Errorf("project = %+v", got)
	}
	if len(got.Items) != 30 {
		t.Errorf("items = %d, want 30", len(got.Items))
	}
	if got.Items[0].URL == "" || got.Items[0].Status == "" || got.Items[0].Number == 0 {
		t.Errorf("first item = %+v", got.Items[0])
	}
}

func TestFetchFiltersByExactTimestamp(t *testing.T) {
	got, _ := fetchSynthetic(t)

	for _, item := range got.Items {
		if item.Number == 2 {
			t.Errorf("item updated before since was kept: %+v", item)
		}
	}
	if len(got.Items) != 5 {
		t.Errorf("items = %d, want 5", len(got.Items))
	}
}

func TestFetchOrdersViewerItemsFirstThenNewest(t *testing.T) {
	got, _ := fetchSynthetic(t)

	titles := make([]string, 0, len(got.Items))
	for _, item := range got.Items {
		titles = append(titles, item.Title)
	}
	want := []string{"PR 3", "Assigned draft", "Issue 1", privateItemTitle, "Draft idea"}
	if !slices.Equal(titles, want) {
		t.Errorf("order = %q, want %q", titles, want)
	}
}

func TestFetchMarksItemsAssignedToViewer(t *testing.T) {
	got, _ := fetchSynthetic(t)

	if !got.Items[0].AssignedToViewer || got.Items[2].AssignedToViewer {
		t.Errorf("items = %+v", got.Items)
	}
	if !slices.Equal(got.Items[0].Assignees, []string{"octocat", "bob"}) {
		t.Errorf("assignees = %v", got.Items[0].Assignees)
	}
}

func TestFetchShowsMissingStatus(t *testing.T) {
	got, _ := fetchSynthetic(t)

	index := slices.IndexFunc(got.Items, func(i Item) bool { return i.Title == "Draft idea" })
	if index < 0 || got.Items[index].Status != noStatus {
		t.Errorf("items = %+v, want the draft with %q", got.Items, noStatus)
	}
}

func TestItemsWithoutOwnURLLinkToTheBoard(t *testing.T) {
	got, _ := fetchSynthetic(t)

	for _, title := range []string{"Draft idea", privateItemTitle} {
		index := slices.IndexFunc(got.Items, func(i Item) bool { return i.Title == title })
		if index < 0 || got.Items[index].URL != got.URL {
			t.Errorf("%q should link to the board: %+v", title, got.Items)
		}
	}
}

func TestItemsAlwaysHaveAnAssigneesList(t *testing.T) {
	got, _ := fetchSynthetic(t)

	for _, item := range got.Items {
		if item.Assignees == nil {
			t.Errorf("%q has a nil list, which encodes as null", item.Title)
		}
	}
}

func TestFetchUsesADateFilterOneDayEarlier(t *testing.T) {
	_, client := fetchSynthetic(t)

	if got := client.variables[0]["filter"]; got != "updated:>=2026-09-30" {
		t.Errorf("filter = %v", got)
	}
	if got := client.variables[0]["owner"]; got != "octocat" {
		t.Errorf("owner = %v", got)
	}
}

func TestFetchEmptyBoardHasEmptyList(t *testing.T) {
	client := syntheticClient(t)

	got, err := FetchWith(context.Background(), client, "octocat", 3, date(t, "2030-01-01T00:00:00Z"), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Items == nil || len(got.Items) != 0 {
		t.Errorf("items = %#v, want an empty list", got.Items)
	}
}

func TestFetchTranslatesMissingScope(t *testing.T) {
	client := &fakeClient{t: t, err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "INSUFFICIENT_SCOPES", Message: "requires one of the following scopes: ['read:project']"}}}}

	_, err := FetchWith(context.Background(), client, "octocat", 3, time.Now(), "octocat")

	if !errors.Is(err, ErrMissingScope) {
		t.Fatalf("error = %v, want ErrMissingScope", err)
	}
	if !strings.Contains(err.Error(), "gh auth refresh -s read:project") {
		t.Errorf("error = %q, want the exact fix", err)
	}
}

func TestFetchExplainsProjectNotFound(t *testing.T) {
	client := &fakeClient{t: t, err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "NOT_FOUND", Message: "Could not resolve to a ProjectV2"}}}}

	_, err := FetchWith(context.Background(), client, "octocat", 9, time.Now(), "octocat")

	if err == nil || errors.Is(err, ErrMissingScope) || !strings.Contains(err.Error(), "project 9 of octocat not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestFetchWrapsOtherErrors(t *testing.T) {
	cause := errors.New("HTTP 502")
	client := &fakeClient{t: t, err: cause}

	if _, err := FetchWith(context.Background(), client, "octocat", 3, time.Now(), "octocat"); !errors.Is(err, cause) {
		t.Fatalf("error = %v, want it to wrap %v", err, cause)
	}
}

func TestConfiguredNumber(t *testing.T) {
	tests := []struct {
		name, fixture string
		want          int
		wantErr       bool
	}{
		{"configured", "synthetic_config_project.json", 3, false},
		{"no file", "synthetic_config_missing.json", 0, false},
		{"invalid YAML", "synthetic_config_invalid.json", 0, true},
		{"negative number", "synthetic_config_negative.json", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{t: t, fixtures: map[string]string{"": tt.fixture}}

			got, err := ConfiguredNumberWith(context.Background(), client, testRepo)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("number = %d, want %d", got, tt.want)
			}
		})
	}
}
