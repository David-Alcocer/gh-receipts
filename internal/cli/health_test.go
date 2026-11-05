package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/David-Alcocer/gh-receipts/internal/config"
	"github.com/David-Alcocer/gh-receipts/internal/health"
)

var healthNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

type healthRun struct {
	configured config.Config
	configErr  error
	fetched    health.Data
	fetchErr   error
	repoErr    error

	repo       repository.Repository
	window     health.Window
	thresholds health.Thresholds
	calls      int

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (r *healthRun) execute(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newHealthCmd(healthDeps{
		currentRepo: func() (repository.Repository, error) { return testRepo, r.repoErr },
		now:         func() time.Time { return healthNow },
		readConfig: func(_ context.Context, repo repository.Repository) (config.Config, error) {
			return r.configured, r.configErr
		},
		fetch: func(_ context.Context, repo repository.Repository, window health.Window, thresholds health.Thresholds) (health.Data, error) {
			r.repo, r.window, r.thresholds = repo, window, thresholds
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

func TestHealthCoversTheLast14Days(t *testing.T) {
	r := &healthRun{}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := health.Window{Since: healthNow.AddDate(0, 0, -14), Until: healthNow}
	if r.repo != testRepo || r.window != want {
		t.Errorf("repo = %+v, window = %+v", r.repo, r.window)
	}
	if !strings.HasPrefix(r.stdout.String(), "Last 14 days") {
		t.Errorf("output = %q", r.stdout.String())
	}
}

func TestHealthDaysFlag(t *testing.T) {
	r := &healthRun{}

	if err := r.execute(t, "--days", "30"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.window.Since.Equal(healthNow.AddDate(0, 0, -30)) {
		t.Errorf("since = %v", r.window.Since)
	}
}

func TestHealthRejectsDaysBelowOne(t *testing.T) {
	for _, days := range []string{"0", "-3"} {
		r := &healthRun{}

		err := r.execute(t, "--days", days)

		if err == nil || !strings.Contains(err.Error(), "--days") || r.calls != 0 {
			t.Errorf("--days %s: error = %v, calls = %d", days, err, r.calls)
		}
	}
}

func TestHealthUsesConfiguredThresholds(t *testing.T) {
	r := &healthRun{configured: config.Config{Health: config.Health{StaleDays: 5}}}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := health.DefaultThresholds
	want.StaleDays = 5
	if r.thresholds != want {
		t.Errorf("thresholds = %+v, want %+v", r.thresholds, want)
	}
}

func TestHealthStopsOnAnInvalidConfig(t *testing.T) {
	cause := errors.New("invalid .github/receipts.yml")
	r := &healthRun{configErr: cause}

	if err := r.execute(t); !errors.Is(err, cause) || r.calls != 0 {
		t.Errorf("error = %v, calls = %d", err, r.calls)
	}
}

func TestHealthJSON(t *testing.T) {
	r := &healthRun{}

	if err := r.execute(t, "--json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var document struct {
		Repository string
		Since      time.Time
	}
	if err := json.Unmarshal(r.stdout.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout.String())
	}
	if document.Repository != "o/r" || !document.Since.Equal(healthNow.AddDate(0, 0, -14)) {
		t.Errorf("document = %+v", document)
	}
}

func TestHealthPrintsWarnings(t *testing.T) {
	r := &healthRun{fetched: health.Data{Warnings: []string{"careful"}}}

	if err := r.execute(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := r.stderr.String(); got != "warning: careful\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestHealthReturnsFetchErrors(t *testing.T) {
	cause := errors.New("boom")
	r := &healthRun{fetchErr: cause}

	if err := r.execute(t); !errors.Is(err, cause) || r.stdout.Len() != 0 {
		t.Errorf("error = %v, output = %q", err, r.stdout.String())
	}
}

func TestHealthRepoFlagWorksOutsideAClone(t *testing.T) {
	r := &healthRun{repoErr: errors.New("not a repo")}

	if err := r.execute(t, "--repo", "acme/app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.repo.Owner != "acme" || r.repo.Name != "app" {
		t.Errorf("repo = %+v", r.repo)
	}
}

func TestHealthOutsideACloneExplainsWhatToDo(t *testing.T) {
	r := &healthRun{repoErr: errors.New("not a repo")}

	err := r.execute(t)

	if err == nil || !strings.Contains(err.Error(), "--repo") || r.calls != 0 {
		t.Errorf("error = %v, calls = %d", err, r.calls)
	}
}

func TestHealthLeavesTheSessionUntouched(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	r := &healthRun{}

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

func TestRootListsHealth(t *testing.T) {
	got, err := run(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "health") {
		t.Errorf("help = %q", got)
	}
}

func TestHealthWindowHasWholeSeconds(t *testing.T) {
	r := &healthRun{}
	cmd := newHealthCmd(healthDeps{
		currentRepo: func() (repository.Repository, error) { return testRepo, nil },
		now:         func() time.Time { return healthNow.Add(123456789) },
		readConfig: func(context.Context, repository.Repository) (config.Config, error) {
			return config.Config{}, nil
		},
		fetch: func(_ context.Context, _ repository.Repository, window health.Window, _ health.Thresholds) (health.Data, error) {
			r.window = window
			return health.Data{}, nil
		},
	})
	cmd.SetOut(&r.stdout)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.window.Until.Equal(healthNow) {
		t.Errorf("until = %v, want %v", r.window.Until, healthNow)
	}
}
