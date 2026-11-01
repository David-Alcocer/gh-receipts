package cli

import (
	"bytes"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newRootCmd("v1.2.3")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestVersionFlagPrintsVersion(t *testing.T) {
	got, err := run(t, "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "gh receipts version v1.2.3\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNoArgsPrintsHelp(t *testing.T) {
	got, err := run(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "gh receipts [flags]") {
		t.Errorf("help output missing usage line:\n%s", got)
	}
}

func TestUnknownArgumentReturnsError(t *testing.T) {
	if _, err := run(t, "nope"); err == nil {
		t.Fatal("expected an error for an unknown argument")
	}
}
