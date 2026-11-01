package criteria

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

var testRepo = repository.Repository{Host: "github.com", Owner: "David-Alcocer", Name: "gh-receipts"}

type fakeClient struct {
	t         *testing.T
	fixture   string
	variables map[string]interface{}
	err       error
}

func (c *fakeClient) DoWithContext(_ context.Context, _ string, variables map[string]interface{}, response interface{}) error {
	c.t.Helper()
	c.variables = variables
	if c.err != nil {
		return c.err
	}
	data, err := os.ReadFile(filepath.Join("testdata", c.fixture))
	if err != nil {
		c.t.Fatal(err)
	}
	var envelope struct{ Data json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		c.t.Fatal(err)
	}
	return json.Unmarshal(envelope.Data, response)
}

func TestFetchRecordedIssue(t *testing.T) {
	client := &fakeClient{t: t, fixture: "issue_5.json"}

	got, err := FetchWith(context.Background(), client, testRepo, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Number != 5 || got.Title != "Acceptance criteria convention and parser" || got.URL != "https://github.com/David-Alcocer/gh-receipts/issues/5" {
		t.Errorf("issue = %+v", got)
	}
	if !got.Found || len(got.Items) != 5 {
		t.Errorf("found = %v, items = %d, want 5", got.Found, len(got.Items))
	}
	if client.variables["owner"] != "David-Alcocer" || client.variables["name"] != "gh-receipts" || client.variables["number"] != 5 {
		t.Errorf("variables = %v", client.variables)
	}
}

// Issue 14 was created with the issue form in .github/ISSUE_TEMPLATE/feature.yml.
func TestFetchRecordedIssueFromTemplate(t *testing.T) {
	client := &fakeClient{t: t, fixture: "issue_14.json"}

	got, err := FetchWith(context.Background(), client, testRepo, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !got.Found || len(got.Items) != 2 {
		t.Fatalf("found = %v, items = %+v", got.Found, got.Items)
	}
	if !strings.HasPrefix(got.Items[0].Text, "`gh receipts commands` lists") || got.Items[1].Text != "`--json` prints the same list" {
		t.Errorf("items = %+v", got.Items)
	}
}

func TestFetchExplainsMissingIssue(t *testing.T) {
	client := &fakeClient{t: t, err: notFound("repository", "issue")}

	_, err := FetchWith(context.Background(), client, testRepo, 13)

	want := "issue 13 of David-Alcocer/gh-receipts not found: check the number, and that it isn't a pull request"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestFetchDoesNotBlameTheNumberForAMissingRepository(t *testing.T) {
	client := &fakeClient{t: t, err: notFound("repository")}

	_, err := FetchWith(context.Background(), client, testRepo, 5)

	if err == nil || strings.Contains(err.Error(), "check the number") || !strings.Contains(err.Error(), "Could not resolve") {
		t.Errorf("error = %v", err)
	}
}

func notFound(path ...interface{}) error {
	return &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "NOT_FOUND", Path: path, Message: "Could not resolve"}}}
}

func TestFetchKeepsOtherErrors(t *testing.T) {
	cause := errors.New("boom")
	client := &fakeClient{t: t, err: cause}

	_, err := FetchWith(context.Background(), client, testRepo, 5)

	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "read issue 5") {
		t.Errorf("error = %v", err)
	}
}

func TestFetchHandlesAnEmptyResponse(t *testing.T) {
	client := &fakeClient{t: t, fixture: "synthetic_no_issue.json"}

	_, err := FetchWith(context.Background(), client, testRepo, 5)

	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v", err)
	}
}
