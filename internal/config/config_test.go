package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

var testRepo = repository.Repository{Host: "github.com", Owner: "octocat", Name: "r"}

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

func TestReadWith(t *testing.T) {
	tests := []struct {
		name, fixture string
		want          Config
	}{
		{"no file", "synthetic_missing.json", Config{}},
		{"project only", "synthetic_project.json", Config{Project: 3}},
		{
			"health thresholds",
			"synthetic_health.json",
			Config{Project: 2, Health: Health{MaxPRSize: 250, MaxFirstReviewHours: 8, MaxMergeHours: 72, StaleDays: 5}},
		},
		{"some thresholds", "synthetic_health_partial.json", Config{Health: Health{StaleDays: 3}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{t: t, fixture: tt.fixture}

			got, err := ReadWith(context.Background(), client, testRepo)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("config = %+v, want %+v", got, tt.want)
			}
			if client.variables["owner"] != "octocat" || client.variables["name"] != "r" {
				t.Errorf("variables = %v", client.variables)
			}
		})
	}
}

func TestReadWithRejectsInvalidFiles(t *testing.T) {
	tests := []struct{ name, fixture, wantInError string }{
		{"invalid YAML", "synthetic_invalid.json", "invalid .github/receipts.yml"},
		{"negative project", "synthetic_negative_project.json", "project must be a positive number, got -1"},
		{"zero threshold", "synthetic_zero_threshold.json", "health.maxMergeHours must be a positive number, got 0"},
		{"negative threshold", "synthetic_negative_threshold.json", "health.staleDays must be a positive number, got -2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{t: t, fixture: tt.fixture}

			_, err := ReadWith(context.Background(), client, testRepo)

			if err == nil || !strings.Contains(err.Error(), tt.wantInError) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantInError)
			}
		})
	}
}

func TestReadWithWrapsRequestErrors(t *testing.T) {
	cause := errors.New("HTTP 502")
	client := &fakeClient{t: t, err: cause}

	if _, err := ReadWith(context.Background(), client, testRepo); !errors.Is(err, cause) {
		t.Errorf("error = %v, want it to wrap %v", err, cause)
	}
}
