package clitest

import (
	"strings"
	"testing"
)

func TestHelpExitsZeroAndNamesTheCLI(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "--help")
	if res.Code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", res.Code, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "eve-fleet") {
		t.Fatalf("help should name eve-fleet, got:\n%s", res.Stdout)
	}
}
