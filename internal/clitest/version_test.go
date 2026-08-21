package clitest

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionPrintsCLIVersion(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "version")
	if res.Code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", res.Code, res.Stderr)
	}
	got := strings.TrimSpace(res.Stdout)
	if got == "" {
		t.Fatal("version printed nothing")
	}
	if strings.Contains(got, "not implemented") {
		t.Fatalf("version stubbed: %s", got)
	}
}

func TestVersionJSON(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "version", "--json")
	if res.Code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", res.Code, res.Stderr)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		t.Fatalf("json: %v\nstdout:\n%s", err, res.Stdout)
	}
	if payload.Version == "" {
		t.Fatalf("missing version field: %s", res.Stdout)
	}
}
