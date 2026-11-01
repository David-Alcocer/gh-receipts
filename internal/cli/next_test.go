package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/pending"
)

type nextRun struct {
	repo     repository.Repository
	calls    int
	fetched  pending.Pending
	fetchErr error
	repoErr  error

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (r *nextRun) execute(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newNextCmd(nextDeps{
		currentRepo: func() (repository.Repository, error) { return testRepo, r.repoErr },
		fetch: func(_ context.Context, repo repository.Repository) (pending.Pending, error) {
			r.repo = repo
			r.calls++
			return r.fetched, r.fetchErr
		},
	})
	cmd.SetOut(&r.stdout)
	cmd.SetErr(&r.stderr)
	cmd.SetArgs(args)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	return cmd.Execute()
}

func TestNextPrintsWhatIsPending(t *testing.T) {
	r := &nextRun{fetched: pending.Pending{Issues: []pending.Issue{{Number: 4, Title: "Task", URL: "https://github.com/o/r/issues/4"}}}}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.repo != testRepo || !strings.Contains(r.stdout.String(), "#4 Task") {
		t.Errorf("repo = %+v, output = %q", r.repo, r.stdout.String())
	}
}

func TestNextJSON(t *testing.T) {
	r := &nextRun{fetched: pending.Pending{Viewer: "octocat"}}

	if err := r.execute(t, "--json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document struct{ Viewer string }
	if err := json.Unmarshal(r.stdout.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout.String())
	}
	if document.Viewer != "octocat" {
		t.Errorf("document = %+v", document)
	}
}

func TestNextRepoFlagWorksOutsideAClone(t *testing.T) {
	r := &nextRun{repoErr: errors.New("not a repo")}

	if err := r.execute(t, "--repo", "acme/app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.repo.Owner != "acme" || r.repo.Name != "app" {
		t.Errorf("repo = %+v", r.repo)
	}
}

func TestNextPrintsWarnings(t *testing.T) {
	r := &nextRun{fetched: pending.Pending{Warnings: []string{"careful"}}}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := r.stderr.String(); got != "warning: careful\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestNextReturnsErrors(t *testing.T) {
	cause := errors.New("boom")
	r := &nextRun{fetchErr: cause}

	if err := r.execute(t); !errors.Is(err, cause) {
		t.Errorf("error = %v", err)
	}
	if r.stdout.Len() != 0 {
		t.Errorf("output = %q", r.stdout.String())
	}
}

func TestNextOutsideACloneExplainsWhatToDo(t *testing.T) {
	r := &nextRun{repoErr: errors.New("not a repo")}

	err := r.execute(t)

	if err == nil || !strings.Contains(err.Error(), "--repo") || r.calls != 0 {
		t.Errorf("error = %v, calls = %d", err, r.calls)
	}
}

func TestNextRejectsArguments(t *testing.T) {
	r := &nextRun{}

	if err := r.execute(t, "5"); err == nil || r.calls != 0 {
		t.Errorf("error = %v, calls = %d", err, r.calls)
	}
}

func TestNextLeavesTheSessionUntouched(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	r := &nextRun{}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("state dir = %v", entries)
	}
}

func TestRootListsNext(t *testing.T) {
	got, err := run(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "next") {
		t.Errorf("help = %q", got)
	}
}
