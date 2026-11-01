package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/criteria"
)

type criteriaRun struct {
	repo     repository.Repository
	number   int
	calls    int
	issue    criteria.Issue
	fetchErr error
	repoErr  error

	stdout bytes.Buffer
}

func (r *criteriaRun) execute(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newCriteriaCmd(criteriaDeps{
		currentRepo: func() (repository.Repository, error) { return testRepo, r.repoErr },
		fetch: func(_ context.Context, repo repository.Repository, number int) (criteria.Issue, error) {
			r.repo, r.number = repo, number
			r.calls++
			return r.issue, r.fetchErr
		},
	})
	cmd.SetOut(&r.stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	return cmd.Execute()
}

func parsedIssue() criteria.Issue {
	return criteria.Issue{
		Number: 5,
		Title:  "Parser",
		State:  "OPEN",
		URL:    "https://github.com/o/r/issues/5",
		Result: criteria.Parse("## Acceptance criteria\n- [x] One\n- [ ] Two\n"),
	}
}

func TestCriteriaPrintsTheIssue(t *testing.T) {
	r := &criteriaRun{issue: parsedIssue()}

	if err := r.execute(t, "5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.repo != testRepo || r.number != 5 {
		t.Errorf("repo = %+v, number = %d", r.repo, r.number)
	}
	if got := r.stdout.String(); !strings.Contains(got, "1/2") || !strings.Contains(got, "[x] One") {
		t.Errorf("output = %q", got)
	}
}

func TestCriteriaJSONFlagAfterTheNumber(t *testing.T) {
	r := &criteriaRun{issue: parsedIssue()}

	if err := r.execute(t, "5", "--json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document struct {
		Number int
		Items  []criteria.Item
	}
	if err := json.Unmarshal(r.stdout.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout.String())
	}
	if document.Number != 5 || len(document.Items) != 2 {
		t.Errorf("document = %+v", document)
	}
}

func TestCriteriaWithoutCriteriaIsNotAnError(t *testing.T) {
	r := &criteriaRun{issue: criteria.Issue{Number: 5, Title: "Parser", URL: "u", Result: criteria.Parse("nothing")}}

	if err := r.execute(t, "5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(r.stdout.String(), "no criteria found") {
		t.Errorf("output = %q", r.stdout.String())
	}
}

func TestCriteriaRepoFlagWorksOutsideAClone(t *testing.T) {
	r := &criteriaRun{issue: parsedIssue(), repoErr: errors.New("not a repo")}

	if err := r.execute(t, "5", "--repo", "acme/app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.repo.Owner != "acme" || r.repo.Name != "app" {
		t.Errorf("repo = %+v", r.repo)
	}
}

func TestCriteriaOutsideACloneExplainsWhatToDo(t *testing.T) {
	r := &criteriaRun{repoErr: errors.New("not a repo")}

	err := r.execute(t, "5")

	if err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Errorf("error = %v", err)
	}
}

func TestCriteriaRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"abc"}, {"0"}, {"--", "-1"}, {"#5"}, {}, {"1", "2"}} {
		r := &criteriaRun{}

		if err := r.execute(t, args...); err == nil {
			t.Errorf("args %v: expected an error", args)
		}
		if r.calls != 0 {
			t.Errorf("args %v: fetched anyway", args)
		}
	}
}

func TestCriteriaInvalidNumberExplainsTheFormat(t *testing.T) {
	r := &criteriaRun{}

	err := r.execute(t, "abc")

	if err == nil || err.Error() != `invalid issue "abc": it must be a positive number` {
		t.Errorf("error = %v", err)
	}
}

func TestCriteriaReturnsFetchErrors(t *testing.T) {
	cause := errors.New("boom")
	r := &criteriaRun{fetchErr: cause}

	if err := r.execute(t, "5"); !errors.Is(err, cause) {
		t.Errorf("error = %v", err)
	}
	if r.stdout.Len() != 0 {
		t.Errorf("output = %q", r.stdout.String())
	}
}

func TestRootListsCriteria(t *testing.T) {
	got, err := run(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "criteria") {
		t.Errorf("help = %q", got)
	}
}
