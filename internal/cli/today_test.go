package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/activity"
	"github.com/David-Alcocer/gh-receipts/internal/project"
)

var (
	testNow  = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	testRepo = repository.Repository{Host: "github.com", Owner: "o", Name: "r"}
)

type fakeStore struct {
	last    time.Time
	found   bool
	readErr error
	saved   []time.Time
	saveErr error
}

func (s *fakeStore) LastRun(repository.Repository) (time.Time, bool, error) {
	return s.last, s.found, s.readErr
}

func (s *fakeStore) Save(_ repository.Repository, at time.Time) error {
	s.saved = append(s.saved, at)
	return s.saveErr
}

type todayRun struct {
	store    *fakeStore
	since    time.Time
	repo     repository.Repository
	fetchErr error
	repoErr  error
	fetched  activity.Activity

	configured    int
	configuredErr error
	board         project.Project
	boardErr      error
	boardNumber   int
	boardOwner    string
	boardViewer   string

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (r *todayRun) execute(t *testing.T, args ...string) error {
	t.Helper()
	if r.store == nil {
		r.store = &fakeStore{}
	}
	cmd := newTodayCmd(todayDeps{
		store: r.store,
		now:   func() time.Time { return testNow },
		currentRepo: func() (repository.Repository, error) {
			return testRepo, r.repoErr
		},
		fetch: func(_ context.Context, repo repository.Repository, since time.Time) (activity.Activity, error) {
			r.repo, r.since = repo, since
			return r.fetched, r.fetchErr
		},
		configuredProject: func(context.Context, repository.Repository) (int, error) {
			return r.configured, r.configuredErr
		},
		fetchProject: func(_ context.Context, owner string, number int, _ time.Time, viewer string) (project.Project, error) {
			r.boardOwner, r.boardNumber, r.boardViewer = owner, number, viewer
			return r.board, r.boardErr
		},
	})
	cmd.SetOut(&r.stdout)
	cmd.SetErr(&r.stderr)
	cmd.SetArgs(args)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	return cmd.Execute()
}

func TestTodayFallsBackTo24HoursOnFirstRun(t *testing.T) {
	run := &todayRun{}
	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := testNow.Add(-24 * time.Hour); !run.since.Equal(want) {
		t.Errorf("since = %v, want %v", run.since, want)
	}
}

func TestTodayUsesAndUpdatesLastRun(t *testing.T) {
	last := testNow.Add(-3 * time.Hour)
	run := &todayRun{store: &fakeStore{last: last, found: true}}

	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !run.since.Equal(last) {
		t.Errorf("since = %v, want %v", run.since, last)
	}
	if len(run.store.saved) != 1 || !run.store.saved[0].Equal(testNow) {
		t.Errorf("saved = %v, want the start of the run", run.store.saved)
	}
}

func TestTodaySinceFlagOverridesWithoutUpdating(t *testing.T) {
	tests := []struct {
		name, value string
		want        time.Time
	}{
		{"duration", "48h", testNow.Add(-48 * time.Hour)},
		{"date", "2026-11-01", time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &todayRun{store: &fakeStore{last: testNow.Add(-time.Hour), found: true}}

			if err := run.execute(t, "--since", tt.value); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !run.since.Equal(tt.want) {
				t.Errorf("since = %v, want %v", run.since, tt.want)
			}
			if len(run.store.saved) != 0 {
				t.Errorf("saved = %v, want nothing", run.store.saved)
			}
		})
	}
}

func TestTodayRejectsInvalidSince(t *testing.T) {
	for _, value := range []string{"yesterday", "2d", "-48h", "2030-01-01"} {
		t.Run(value, func(t *testing.T) {
			run := &todayRun{}
			err := run.execute(t, "--since", value)
			if err == nil || !strings.Contains(err.Error(), "--since") {
				t.Fatalf("error = %v, want one that mentions --since", err)
			}
		})
	}
}

func TestTodayJSONDoesNotUpdateLastRun(t *testing.T) {
	run := &todayRun{store: &fakeStore{last: testNow.Add(-time.Hour), found: true}}

	if err := run.execute(t, "--json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(run.store.saved) != 0 {
		t.Errorf("saved = %v, want nothing", run.store.saved)
	}
	var document struct{ SchemaVersion int }
	if err := json.Unmarshal(run.stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if document.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", document.SchemaVersion)
	}
}

func TestTodayKeepsLastRunWhenFetchFails(t *testing.T) {
	cause := errors.New("HTTP 502")
	run := &todayRun{fetchErr: cause, store: &fakeStore{last: testNow.Add(-time.Hour), found: true}}

	err := run.execute(t)

	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want it to wrap %v", err, cause)
	}
	if len(run.store.saved) != 0 {
		t.Errorf("saved = %v, want nothing", run.store.saved)
	}
}

func TestTodayRepoFlagWorksOutsideARepository(t *testing.T) {
	run := &todayRun{repoErr: errors.New("not a git repository")}

	if err := run.execute(t, "--repo", "cli/cli"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.repo.Owner != "cli" || run.repo.Name != "cli" {
		t.Errorf("repo = %+v, want cli/cli", run.repo)
	}
}

func TestTodayRejectsInvalidRepoFlag(t *testing.T) {
	run := &todayRun{}
	err := run.execute(t, "--repo", "not-a-repo")
	if err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("error = %v, want one that mentions --repo", err)
	}
}

func TestTodayOutsideARepositoryExplainsTheFix(t *testing.T) {
	run := &todayRun{repoErr: errors.New("not a git repository")}

	err := run.execute(t)

	if err == nil || !strings.Contains(err.Error(), "--repo owner/name") {
		t.Fatalf("error = %v, want one that explains --repo owner/name", err)
	}
}

func TestTodayPrintsWarningsToStderr(t *testing.T) {
	run := &todayRun{fetched: activity.Activity{Warnings: []string{"results are incomplete"}}}

	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(run.stderr.String(), "results are incomplete") {
		t.Errorf("stderr = %q", run.stderr.String())
	}
	if strings.Contains(run.stdout.String(), "results are incomplete") {
		t.Errorf("warning leaked into stdout: %q", run.stdout.String())
	}
}

func TestTodayFallsBackWhenStoredTimeIsUnreadable(t *testing.T) {
	run := &todayRun{store: &fakeStore{readErr: errors.New("corrupt file")}}

	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := testNow.Add(-24 * time.Hour); !run.since.Equal(want) {
		t.Errorf("since = %v, want %v", run.since, want)
	}
	if !strings.Contains(run.stderr.String(), "corrupt file") {
		t.Errorf("stderr = %q, want a warning", run.stderr.String())
	}
}

func TestTodayWarnsWhenLastRunCannotBeSaved(t *testing.T) {
	run := &todayRun{store: &fakeStore{saveErr: errors.New("disk full")}}

	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(run.stderr.String(), "disk full") {
		t.Errorf("stderr = %q, want a warning", run.stderr.String())
	}
}

func TestTodayWithoutProjectSkipsTheBoard(t *testing.T) {
	run := &todayRun{}
	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.boardNumber != 0 {
		t.Errorf("board %d was fetched", run.boardNumber)
	}
}

func TestTodayProjectFlagBeatsConfiguration(t *testing.T) {
	run := &todayRun{configured: 3, fetched: activity.Activity{Viewer: "alice"}}

	if err := run.execute(t, "--project", "7"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.boardNumber != 7 || run.boardOwner != "o" || run.boardViewer != "alice" {
		t.Errorf("fetched board %d of %q for %q", run.boardNumber, run.boardOwner, run.boardViewer)
	}
}

func TestTodayUsesConfiguredProject(t *testing.T) {
	run := &todayRun{configured: 3}
	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.boardNumber != 3 {
		t.Errorf("board = %d, want 3", run.boardNumber)
	}
}

func TestTodayRejectsInvalidProjectFlag(t *testing.T) {
	for _, value := range []string{"0", "-1"} {
		run := &todayRun{}
		err := run.execute(t, "--project", value)
		if err == nil || !strings.Contains(err.Error(), "--project") {
			t.Errorf("%s: error = %v, want one that mentions --project", value, err)
		}
	}
}

func TestTodayMissingScopeKeepsTheReportAndTheSession(t *testing.T) {
	run := &todayRun{
		configured: 3,
		boardErr:   project.ErrMissingScope,
		store:      &fakeStore{last: testNow.Add(-time.Hour), found: true},
		fetched:    activity.Activity{MergedPRs: []activity.PullRequest{{Number: 1, Title: "One", Author: "bob", URL: "https://x/1"}}},
	}

	if err := run.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(run.stdout.String(), "MERGED") {
		t.Errorf("the report is missing: %q", run.stdout.String())
	}
	if !strings.Contains(run.stderr.String(), "gh auth refresh -s read:project") {
		t.Errorf("stderr = %q, want the exact fix", run.stderr.String())
	}
	if len(run.store.saved) != 0 {
		t.Errorf("saved = %v, want nothing", run.store.saved)
	}
}

func TestTodayStopsOnOtherProjectErrors(t *testing.T) {
	cause := errors.New("project not found")
	run := &todayRun{configured: 9, boardErr: cause}

	if err := run.execute(t); !errors.Is(err, cause) {
		t.Fatalf("error = %v, want it to wrap %v", err, cause)
	}
	if len(run.store.saved) != 0 {
		t.Errorf("saved = %v, want nothing", run.store.saved)
	}
}

func TestTodayStopsWhenConfigurationIsInvalid(t *testing.T) {
	cause := errors.New("invalid receipts.yml")
	run := &todayRun{configuredErr: cause}

	if err := run.execute(t); !errors.Is(err, cause) {
		t.Fatalf("error = %v, want it to wrap %v", err, cause)
	}
}

func TestTodayJSONIncludesTheProject(t *testing.T) {
	run := &todayRun{configured: 3, board: project.Project{Number: 3, Title: "Board", Items: []project.Item{}}}

	if err := run.execute(t, "--json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(run.stdout.String(), `"title": "Board"`) {
		t.Errorf("stdout = %s", run.stdout.String())
	}
}
