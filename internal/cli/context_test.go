package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/pending"
	"github.com/David-Alcocer/gh-receipts/internal/project"
)

type contextRun struct {
	store      *fakeStore
	repoErr    error
	timeout    time.Duration
	since      time.Time
	repo       repository.Repository
	fetched    activity.Activity
	fetchErr   error
	pending    pending.Pending
	pendingErr error
	configured int
	board      project.Project
	boardErr   error
	// block makes both fetches wait until the deadline.
	block bool

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (r *contextRun) execute(t *testing.T, args ...string) error {
	t.Helper()
	if r.store == nil {
		r.store = &fakeStore{}
	}
	if r.timeout == 0 {
		r.timeout = time.Minute
	}
	wait := func(ctx context.Context) error {
		if !r.block {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}
	cmd := newContextCmd(contextDeps{
		today: todayDeps{
			store:       r.store,
			now:         func() time.Time { return testNow },
			currentRepo: func() (repository.Repository, error) { return testRepo, r.repoErr },
			fetch: func(ctx context.Context, repo repository.Repository, since time.Time) (activity.Activity, error) {
				r.repo, r.since = repo, since
				if err := wait(ctx); err != nil {
					return activity.Activity{}, err
				}
				return r.fetched, r.fetchErr
			},
			configuredProject: func(context.Context, repository.Repository) (int, error) {
				return r.configured, nil
			},
			fetchProject: func(context.Context, string, int, time.Time, string) (project.Project, error) {
				return r.board, r.boardErr
			},
		},
		fetchPending: func(ctx context.Context, _ repository.Repository) (pending.Pending, error) {
			if err := wait(ctx); err != nil {
				return pending.Pending{}, err
			}
			return r.pending, r.pendingErr
		},
		timeout: r.timeout,
	})
	cmd.SetOut(&r.stdout)
	cmd.SetErr(&r.stderr)
	cmd.SetArgs(args)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	return cmd.Execute()
}

func samplePendingWork() pending.Pending {
	return pending.Pending{Issues: []pending.Issue{{Number: 4, Title: "Task", URL: "https://github.com/o/r/issues/4"}}}
}

func sampleChanges() activity.Activity {
	return activity.Activity{MergedPRs: []activity.PullRequest{{Number: 9, Title: "Done", Author: "a", URL: "https://github.com/o/r/pull/9"}}}
}

func TestContextShowsChangesAndPendingWork(t *testing.T) {
	r := &contextRun{fetched: sampleChanges(), pending: samplePendingWork()}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := r.stdout.String()
	if r.repo != testRepo || !strings.Contains(got, "#4 Task") || !strings.Contains(got, "#9 Done") || r.stderr.Len() != 0 {
		t.Errorf("repo = %+v, stdout = %q, stderr = %q", r.repo, got, r.stderr.String())
	}
}

func TestContextNeverMovesTheSession(t *testing.T) {
	last := testNow.Add(-3 * time.Hour)
	r := &contextRun{store: &fakeStore{last: last, found: true}, fetched: sampleChanges(), pending: samplePendingWork()}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.since.Equal(last) || len(r.store.saved) != 0 {
		t.Errorf("since = %v, saved = %v", r.since, r.store.saved)
	}
	if !strings.Contains(r.stdout.String(), "(last session)") {
		t.Errorf("stdout = %q", r.stdout.String())
	}
}

func TestContextFallsBackTo24HoursOnFirstRun(t *testing.T) {
	r := &contextRun{}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := testNow.Add(-24 * time.Hour); !r.since.Equal(want) {
		t.Errorf("since = %v, want %v", r.since, want)
	}
}

func TestContextNotesAnUnreadableSession(t *testing.T) {
	r := &contextRun{store: &fakeStore{readErr: errors.New("bad session file")}}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(r.stdout.String(), "Note: bad session file") || r.stderr.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q", r.stdout.String(), r.stderr.String())
	}
}

func TestContextNeverFailsOutsideARepository(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no clone", nil, "gh receipts context: no GitHub repository found here"},
		{"invalid flag value", []string{"--repo", "nope"}, "gh receipts context: invalid --repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &contextRun{repoErr: errors.New("none")}

			if err := r.execute(t, tt.args...); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := r.stdout.String()
			if !strings.HasPrefix(got, tt.want) || strings.Count(got, "\n") != 1 {
				t.Errorf("stdout = %q", got)
			}
		})
	}
}

func TestContextShowsWhatItReadWhenOneFetchFails(t *testing.T) {
	tests := []struct {
		name       string
		run        *contextRun
		shown      string
		missing    string
		wantInNote string
	}{
		{
			name:       "changes fail",
			run:        &contextRun{fetchErr: errors.New("HTTP 502"), pending: samplePendingWork()},
			shown:      "#4 Task",
			wantInNote: "Note: couldn't read what changed: HTTP 502",
		},
		{
			name:       "pending work fails",
			run:        &contextRun{fetched: sampleChanges(), pendingErr: errors.New("HTTP 502")},
			shown:      "#9 Done",
			wantInNote: "Note: couldn't read what is waiting for you: HTTP 502",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run.execute(t); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := tt.run.stdout.String()
			if !strings.Contains(got, tt.shown) || !strings.Contains(got, tt.wantInNote) {
				t.Errorf("stdout = %q", got)
			}
		})
	}
}

func TestContextPrintsOneLineWhenNothingCouldBeRead(t *testing.T) {
	r := &contextRun{fetchErr: errors.New("HTTP 401: Bad credentials"), pendingErr: errors.New("HTTP 401: Bad credentials")}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := r.stdout.String()
	if !strings.HasPrefix(got, "gh receipts context: couldn't read GitHub: HTTP 401: Bad credentials") || strings.Count(got, "\n") != 1 {
		t.Errorf("stdout = %q", got)
	}
}

func TestContextStopsWaitingAtTheTimeout(t *testing.T) {
	r := &contextRun{block: true, timeout: 10 * time.Millisecond}

	done := make(chan error, 1)
	go func() { done <- r.execute(t) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("context didn't stop at the timeout")
	}
	if got := r.stdout.String(); !strings.Contains(got, "took more than 10ms") {
		t.Errorf("stdout = %q", got)
	}
}

func TestContextPrintsWarningsAsNotes(t *testing.T) {
	changes := sampleChanges()
	changes.Warnings = []string{"search was cut"}
	work := samplePendingWork()
	work.Warnings = []string{"criteria cut"}
	r := &contextRun{fetched: changes, pending: work}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := r.stdout.String()
	if !strings.Contains(got, "Note: search was cut") || !strings.Contains(got, "Note: criteria cut") || r.stderr.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q", got, r.stderr.String())
	}
}

func TestContextIncludesTheProject(t *testing.T) {
	board := project.Project{URL: "https://github.com/orgs/o/projects/1", Items: []project.Item{{Number: 3, Title: "Card", Status: "Done", URL: "https://github.com/o/r/issues/3"}}}
	r := &contextRun{configured: 1, board: board}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := r.stdout.String(); !strings.Contains(got, "PROJECT") || !strings.Contains(got, "#3 Card") {
		t.Errorf("stdout = %q", got)
	}
}

func TestContextNotesAMissingProjectScope(t *testing.T) {
	r := &contextRun{configured: 1, boardErr: project.ErrMissingScope, fetched: sampleChanges()}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := r.stdout.String()
	if !strings.Contains(got, "Note: "+project.ErrMissingScope.Error()) || !strings.Contains(got, "#9 Done") {
		t.Errorf("stdout = %q", got)
	}
}

func TestContextRejectsArguments(t *testing.T) {
	if err := (&contextRun{}).execute(t, "extra"); err == nil {
		t.Error("expected an error")
	}
}

func TestRootListsContext(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\n  context ") {
		t.Errorf("help = %q", out)
	}
}
