package clitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevWithoutSupervisorIsBestEffortAndVisibleInStatus(t *testing.T) {
	root := seededFleet(t)
	dev := Run(t, root, nil, "dev", "--json")
	if dev.Code != 0 {
		t.Fatalf("dev: %s%s", dev.Stdout, dev.Stderr)
	}
	body, err := os.ReadFile(filepath.Join(root, ".eve-fleet", "dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Supervisor  bool     `json:"supervisor"`
		Degradation []string `json:"degradation"`
		Eve         string   `json:"eve"`
	}
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Supervisor {
		t.Fatal("default supervisor is false")
	}
	if len(rec.Degradation) < 3 {
		t.Fatalf("expected degradation list: %+v", rec)
	}
	if !strings.Contains(rec.Eve, "eve") {
		t.Fatalf("expected stock eve compose note: %s", rec.Eve)
	}
	st := Run(t, root, nil, "status")
	if st.Code != 0 {
		t.Fatalf("status: %s%s", st.Stdout, st.Stderr)
	}
	out := st.Stdout
	for _, want := range []string{"supervisor=false", "best-effort", "audit-only"} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q\n%s", want, out)
		}
	}
}

func TestDevFailsWithoutFleetfile(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "dev")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "fleetfile.missing") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}
