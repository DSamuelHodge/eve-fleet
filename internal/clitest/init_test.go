package clitest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestInitRequiresName(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "init")
	if res.Code == 0 {
		t.Fatal("init without name should fail")
	}
	out := res.Stdout + res.Stderr
	if !strings.Contains(out, "init.name.required") || !strings.Contains(out, "suggestion:") {
		t.Fatalf("expected path/rule/suggestion diagnostic, got:\n%s", out)
	}
}

func TestInitRejectsInvalidDNSLabel(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "init", "Revenue_Ops")
	if res.Code == 0 {
		t.Fatal("expected non-zero exit for invalid name")
	}
	out := res.Stdout + res.Stderr
	if !strings.Contains(out, "metadata.name") && !strings.Contains(out, "dns") && !strings.Contains(strings.ToLower(out), "name") {
		t.Fatalf("diagnostic should mention the name rule, got:\n%s", out)
	}
	if !strings.Contains(out, "suggestion") && !strings.Contains(out, "→") && !strings.Contains(out, "lowercase") {
		t.Fatalf("diagnostic should include a suggestion, got:\n%s", out)
	}
}

func TestInitScaffoldsGitFleetAndDoctorPasses(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "init", "revenue-ops")
	if res.Code != 0 {
		t.Fatalf("init exit %d\nstdout:\n%s\nstderr:\n%s", res.Code, res.Stdout, res.Stderr)
	}
	root := filepath.Join(dir, "revenue-ops")

	sha := gitSHA(t, root)
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(sha) {
		t.Fatalf("expected 40-char commit SHA, got %q", sha)
	}

	body, err := os.ReadFile(filepath.Join(root, "Fleetfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"apiVersion: eve.fleet/v1",
		"kind: Fleet",
		"name: revenue-ops",
		"version:",
		"isolation: strong",
		"supervisor: false",
		"required: true",
		"pin: commit",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Fleetfile missing %q\n%s", want, text)
		}
	}
	if !strings.Contains(text, "agents: true") || !strings.Contains(text, "shared: true") {
		t.Errorf("hot_load defaults missing:\n%s", text)
	}

	for _, p := range []string{
		"agents",
		"shared/skills",
		"shared/tools",
		"shared/connections",
		"shared/lib",
		"evals",
		".eve-fleet",
	} {
		if st, err := os.Stat(filepath.Join(root, p)); err != nil || !st.IsDir() {
			t.Errorf("missing conventional dir %s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Fleet.lock")); err == nil {
		t.Error("Fleet.lock must not exist")
	}
	if _, err := os.Stat(filepath.Join(root, "VERSION")); err == nil {
		t.Error("root VERSION file must not exist")
	}

	doc := Run(t, root, nil, "doctor")
	if doc.Code != 0 {
		t.Fatalf("doctor exit %d\nstdout:\n%s\nstderr:\n%s", doc.Code, doc.Stdout, doc.Stderr)
	}
	if strings.Contains(doc.Stdout, "Initialized") {
		t.Fatalf("doctor must not print init copy: %s", doc.Stdout)
	}
	if !strings.Contains(doc.Stdout, "ok") || !strings.Contains(doc.Stdout, "revenue-ops") {
		t.Fatalf("doctor plain: %s", doc.Stdout)
	}
}

func TestInitJSONIncludesGitSHA(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "init", "ops-core", "--json")
	if res.Code != 0 {
		t.Fatalf("init --json exit %d\nstderr:\n%s\nstdout:\n%s", res.Code, res.Stderr, res.Stdout)
	}
	var payload struct {
		Name   string `json:"name"`
		GitSHA string `json:"gitSHA"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, res.Stdout)
	}
	if payload.Name != "ops-core" {
		t.Fatalf("name: %q", payload.Name)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(payload.GitSHA) {
		t.Fatalf("gitSHA: %q", payload.GitSHA)
	}
}

func gitSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}
