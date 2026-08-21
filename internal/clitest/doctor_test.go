package clitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDoctorJSONOnFreshFleet(t *testing.T) {
	dir := t.TempDir()
	if res := Run(t, dir, nil, "init", "revenue-ops", "--json"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	res := Run(t, filepath.Join(dir, "revenue-ops"), nil, "doctor", "--json")
	if res.Code != 0 {
		t.Fatalf("exit %d\n%s\n%s", res.Code, res.Stdout, res.Stderr)
	}
	var payload struct {
		OK              bool   `json:"ok"`
		Name            string `json:"name"`
		GitSHA          string `json:"gitSHA"`
		TopologyVersion string `json:"topologyVersion"`
		Diagnostics     []any  `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, res.Stdout)
	}
	if !payload.OK {
		t.Fatalf("expected ok: %s", res.Stdout)
	}
	if payload.Name != "revenue-ops" || payload.TopologyVersion != "0.1.0" {
		t.Fatalf("identity: %+v", payload)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(payload.GitSHA) {
		t.Fatalf("gitSHA %q", payload.GitSHA)
	}
}

func TestDoctorMissingFleetfileIsActionable(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "doctor")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	out := res.Stdout + res.Stderr
	for _, want := range []string{"fleetfile.missing", "suggestion:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDoctorRejectsBadAPIVersion(t *testing.T) {
	root := initFleet(t, "bad-api")
	ff := filepath.Join(root, "Fleetfile")
	body, err := os.ReadFile(ff)
	if err != nil {
		t.Fatal(err)
	}
	mut := strings.Replace(string(body), "apiVersion: eve.fleet/v1", "apiVersion: eve.fleet/v0", 1)
	if err := os.WriteFile(ff, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Stdout, "fleetfile.apiVersion") {
		t.Fatalf("expected apiVersion rule:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "suggestion") {
		t.Fatalf("expected suggestion:\n%s", res.Stdout)
	}
}

func TestDoctorRejectsFleetLockAndVersionFile(t *testing.T) {
	root := initFleet(t, "locked")
	if err := os.WriteFile(filepath.Join(root, "Fleet.lock"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	out := res.Stdout
	if !strings.Contains(out, "fleet.lock.forbidden") {
		t.Errorf("missing Fleet.lock rule:\n%s", out)
	}
	if !strings.Contains(out, "version.file.forbidden") {
		t.Errorf("missing VERSION rule:\n%s", out)
	}
}

func initFleet(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	res := Run(t, dir, nil, "init", name, "--json")
	if res.Code != 0 {
		t.Fatalf("init: %s%s", res.Stdout, res.Stderr)
	}
	return filepath.Join(dir, name)
}

func TestDoctorAcceptsOmittedRuntimeDefaults(t *testing.T) {
	root := initFleet(t, "defaults")
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	// strip runtime: block
	text := string(body)
	idx := strings.Index(text, "\nruntime:")
	if idx < 0 {
		t.Fatal("runtime block missing from scaffold")
	}
	if err := os.WriteFile(ff, []byte(text[:idx]+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code != 0 {
		t.Fatalf("defaults should pass doctor:\n%s%s", res.Stdout, res.Stderr)
	}
}

func TestDoctorRejectsMissingAgentsMap(t *testing.T) {
	root := initFleet(t, "no-agents")
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	text := strings.Replace(string(body), "agents: {}\n", "", 1)
	if err := os.WriteFile(ff, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Stdout, "fleetfile.agents.required") {
		t.Fatalf("got %s", res.Stdout)
	}
}
