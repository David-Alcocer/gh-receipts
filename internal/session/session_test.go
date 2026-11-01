package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
)

var testRepo = repository.Repository{Host: "github.com", Owner: "o", Name: "r"}

func TestDirUsesXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state")

	got, err := Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join("/tmp/state", "gh-receipts"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestDirFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/tmp/home")

	got, err := Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join("/tmp/home", ".local", "state", "gh-receipts"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestLastRunOnFirstRun(t *testing.T) {
	store := Store{Dir: t.TempDir()}

	_, found, err := store.LastRun(testRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected no stored time")
	}
}

func TestSaveThenLastRun(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "nested")}
	want := time.Date(2026, 11, 1, 10, 30, 0, 0, time.UTC)

	if err := store.Save(testRepo, want); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, found, err := store.LastRun(testRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found || !got.Equal(want) {
		t.Errorf("LastRun() = %v, %t, want %v", got, found, want)
	}
}

func TestRunsAreStoredPerRepo(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	other := repository.Repository{Host: "github.com", Owner: "o", Name: "other"}

	if err := store.Save(testRepo, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found, _ := store.LastRun(other); found {
		t.Error("a run in one repo leaked into another")
	}
}

func TestSaveNeverLeavesTheStoreDirectory(t *testing.T) {
	root := t.TempDir()
	store := Store{Dir: filepath.Join(root, "store")}
	hostile := repository.Repository{Host: "github.com", Owner: "..", Name: "../../escape"}

	if err := store.Save(hostile, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "store" {
		t.Errorf("root contains %v, want only the store directory", entries)
	}
}

func TestSavedFileIsPrivateAndLeavesNoTemporaryFiles(t *testing.T) {
	store := Store{Dir: t.TempDir()}

	if err := store.Save(testRepo, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("store contains %d files, want 1", len(entries))
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

func TestLastRunReportsCorruptFile(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Save(testRepo, time.Now()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, entries[0].Name()), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.LastRun(testRepo); err == nil {
		t.Fatal("expected an error for a corrupt file")
	}
}
