// Package session remembers when gh-receipts last ran in each repository. The
// data lives outside the working tree, in the XDG state directory.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
)

const appName = "gh-receipts"

// Store keeps the time of the last run per repository. An empty Dir means the
// default directory.
type Store struct {
	Dir string
}

type record struct {
	LastRun time.Time `json:"lastRun"`
}

// Dir returns the default state directory.
func Dir() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find the state directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", appName), nil
}

// LastRun returns the stored time for repo, if there is one.
func (s Store) LastRun(repo repository.Repository) (time.Time, bool, error) {
	path, err := s.path(repo)
	if err != nil {
		return time.Time{}, false, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read the last run: %w", err)
	}

	var stored record
	if err := json.Unmarshal(data, &stored); err != nil {
		return time.Time{}, false, fmt.Errorf("read the last run from %s: %w", path, err)
	}
	return stored.LastRun, true, nil
}

// Save stores the time of a run. It writes to a temporary file first, so an
// interrupted save can't damage the previous value.
func (s Store) Save(repo repository.Repository, at time.Time) error {
	path, err := s.path(repo)
	if err != nil {
		return err
	}
	data, err := json.Marshal(record{LastRun: at})
	if err != nil {
		return fmt.Errorf("encode the last run: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create the state directory: %w", err)
	}
	file, err := os.CreateTemp(dir, ".last-run-*")
	if err != nil {
		return fmt.Errorf("save the last run: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("save the last run: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("save the last run: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("save the last run: %w", err)
	}
	return nil
}

// path escapes the repository into a single file name, so a value from --repo
// can't point outside the directory.
func (s Store) path(repo repository.Repository) (string, error) {
	dir := s.Dir
	if dir == "" {
		defaultDir, err := Dir()
		if err != nil {
			return "", err
		}
		dir = defaultDir
	}
	name := url.QueryEscape(repo.Host + "/" + repo.Owner + "/" + repo.Name)
	return filepath.Join(dir, name+".json"), nil
}
