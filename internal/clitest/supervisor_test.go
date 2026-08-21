package clitest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevenueOpsExampleDoctorAndDev(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "init", "revenue-ops", "--example=revenue-ops")
	if res.Code != 0 {
		t.Fatalf("init: %s%s", res.Stdout, res.Stderr)
	}
	root := filepath.Join(dir, "revenue-ops")
	doc := Run(t, root, nil, "doctor", "--json")
	if doc.Code != 0 {
		t.Fatalf("doctor: %s%s", doc.Stdout, doc.Stderr)
	}
	ff, _ := os.ReadFile(filepath.Join(root, "Fleetfile"))
	s := string(ff)
	for _, want := range []string{
		"lead-intake", "dedupe", "enrich", "score", "route",
		"dedupe_lead", "enrich_lead", "score_lead", "route_lead",
		"requires_ack: true", "supervisor: true",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Fleetfile missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "agents/lead-intake/agent/tools/fleet/ack_edge_route_lead.ts")); err != nil {
		t.Fatal("missing ack tool for route_lead")
	}
	st := Run(t, root, nil, "status")
	if st.Code != 0 {
		t.Fatal(st.Stderr)
	}
	if !strings.Contains(st.Stdout, "supervisor=true") {
		t.Fatalf("status: %s", st.Stdout)
	}
	if strings.Contains(st.Stdout, "degradation") {
		t.Fatalf("supervisor on must not degrade: %s", st.Stdout)
	}
	dev := Run(t, root, nil, "dev")
	if dev.Code != 0 {
		t.Fatalf("dev: %s%s", dev.Stdout, dev.Stderr)
	}
	audit, err := os.ReadFile(filepath.Join(root, ".eve-fleet", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	a := string(audit)
	for _, want := range []string{
		`"kind":"handoff"`, `"payloadHash":`, `"requiresAck":true`,
		"ack_pending", "outcome.blocked", "timeout=",
	} {
		if !strings.Contains(a, want) {
			t.Errorf("audit missing %q in %s", want, a)
		}
	}
}

func TestSupervisorRetryAndReloadRefusal(t *testing.T) {
	root := seededFleet(t)
	if res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead", "--from=lead-intake", "--to=dedupe",
		"--contract=c", "--on-failure=retry", "--timeout=15s"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	mut := strings.Replace(string(body), "supervisor: false", "supervisor: true", 1)
	if mut == string(body) {
		mut = string(body) + "\nruntime:\n  supervisor: true\n"
	}
	if err := os.WriteFile(ff, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "supervisor")
	if res := Run(t, root, nil, "deploy"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	dev := Run(t, root, nil, "dev")
	if dev.Code != 0 {
		t.Fatalf("dev: %s%s", dev.Stdout, dev.Stderr)
	}
	audit, _ := os.ReadFile(filepath.Join(root, ".eve-fleet", "audit.jsonl"))
	if !strings.Contains(string(audit), `"status":"retry"`) {
		t.Fatalf("expected retry attempts: %s", audit)
	}
	body, _ = os.ReadFile(ff)
	if err := os.WriteFile(ff, []byte(strings.Replace(string(body), "contract: c", "contract: changed", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := Run(t, root, nil, "reload", "--agent=lead-intake")
	if ref.Code == 0 {
		t.Fatal("expected topology refuse with supervisor on")
	}
	if !strings.Contains(ref.Stdout+ref.Stderr, "reload.topology") {
		t.Fatalf("got %s%s", ref.Stdout, ref.Stderr)
	}
}
