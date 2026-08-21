package clitest

import (
	"strings"
	"testing"
)

func TestEveryCommandHasHelp(t *testing.T) {
	dir := t.TempDir()
	commands := [][]string{
		{"--help"},
		{"init", "--help"},
		{"agent", "--help"},
		{"agent", "add", "--help"},
		{"edge", "--help"},
		{"edge", "add", "--help"},
		{"shared", "--help"},
		{"shared", "add", "--help"},
		{"doctor", "--help"},
		{"dev", "--help"},
		{"build", "--help"},
		{"link", "--help"},
		{"deploy", "--help"},
		{"reload", "--help"},
		{"status", "--help"},
		{"audit", "--help"},
		{"version", "--help"},
	}
	for _, args := range commands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			res := Run(t, dir, nil, args...)
			if res.Code != 0 {
				t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", res.Code, res.Stdout, res.Stderr)
			}
			out := res.Stdout + res.Stderr
			if !strings.Contains(strings.ToLower(out), "usage") && !strings.Contains(out, "eve-fleet") {
				t.Fatalf("expected help text, got:\n%s", out)
			}
		})
	}
}

func TestGlobalFlagsAreRecognized(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "version", "--json", "--yes", "--non-interactive", "--revision=deadbeef", "--agent=lead-intake", "--shared")
	if res.Code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s\nstdout:\n%s", res.Code, res.Stderr, res.Stdout)
	}
	if strings.Contains(res.Stderr, "unknown flag") {
		t.Fatalf("global flags rejected: %s", res.Stderr)
	}
}
