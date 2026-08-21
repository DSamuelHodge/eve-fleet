package clitest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReloadRequiresTarget(t *testing.T) {
	root := seededFleet(t)
	commitAll(t, root, "seed")
	if res := Run(t, root, nil, "deploy"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	res := Run(t, root, nil, "reload")
	if res.Code == 0 {
		t.Fatal("expected missing target")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "reload.target") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}

func TestReloadRefusesTopologyChange(t *testing.T) {
	root := seededFleet(t)
	if res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead", "--from=lead-intake", "--to=dedupe", "--contract=c"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	commitAll(t, root, "topology")
	if res := Run(t, root, nil, "deploy"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	ff := filepath.Join(root, "Fleetfile")
	body, err := os.ReadFile(ff)
	if err != nil {
		t.Fatal(err)
	}
	mut := strings.Replace(string(body), "contract: c", "contract: changed", 1)
	if mut == string(body) {
		t.Fatalf("could not mutate contract:\n%s", body)
	}
	if err := os.WriteFile(ff, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "reload", "--agent=lead-intake")
	if res.Code == 0 {
		t.Fatal("expected topology refuse")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "reload.topology") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
	audit, err := os.ReadFile(filepath.Join(root, ".eve-fleet", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(audit)
	if !strings.Contains(s, "reload.refused") || !strings.Contains(s, "fromSHA") {
		t.Fatalf("audit missing from/to: %s", s)
	}
}

func TestReloadAllowsImplementationPatch(t *testing.T) {
	root := seededFleet(t)
	commitAll(t, root, "seed")
	if res := Run(t, root, nil, "deploy"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	tool := filepath.Join(root, "agents/lead-intake/agent/tools/custom.ts")
	if err := os.MkdirAll(filepath.Dir(tool), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "reload", "--agent=lead-intake", "--json")
	if res.Code != 0 {
		t.Fatalf("reload: %s%s", res.Stdout, res.Stderr)
	}
	if strings.Contains(res.Stdout, `"ok":false`) {
		t.Fatal(res.Stdout)
	}
}

func TestReloadRequiresDeploy(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "reload", "--agent=lead-intake")
	if res.Code == 0 {
		t.Fatal("expected missing pin")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "reload.pin") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}
