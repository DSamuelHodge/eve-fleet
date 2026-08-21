package clitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusShowsDriftAndSupervisor(t *testing.T) {
	root := seededFleet(t)
	commitAll(t, root, "seed")
	if res := Run(t, root, nil, "deploy"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	st := Run(t, root, nil, "status")
	if st.Code != 0 {
		t.Fatal(st.Stderr)
	}
	for _, want := range []string{"topology=", "git=", "deployed=", "drift=false", "supervisor=false"} {
		if !strings.Contains(st.Stdout, want) {
			t.Errorf("missing %q in %s", want, st.Stdout)
		}
	}
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	if err := os.WriteFile(ff, append(body, []byte("\n# drift\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted := Run(t, root, nil, "status")
	if !strings.Contains(drifted.Stdout, "drift=true") {
		t.Fatalf("expected drift: %s", drifted.Stdout)
	}
}

func TestAuditReconstructsChainWithoutPayload(t *testing.T) {
	root := seededFleet(t)
	commitAll(t, root, "seed")
	if res := Run(t, root, nil, "deploy"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	rec := map[string]any{
		"kind":            "handoff",
		"fleet":           "ops",
		"topologyVersion": "0.1.0",
		"gitSHA":          "abc",
		"edgeName":        "dedupe_lead",
		"from":            "lead-intake",
		"to":              "dedupe",
		"actor":           "lead-intake",
		"payloadHash":     "deadbeef",
		"status":          "ok",
		"requiresAck":     false,
		"acked":           false,
		"started":         "2026-01-01T00:00:00Z",
		"completed":       "2026-01-01T00:00:01Z",
		"outcomeId":       "out-1",
		"message":         "SECRET_PAYLOAD_DO_NOT_ECHO",
	}
	b, _ := json.Marshal(rec)
	if err := os.MkdirAll(filepath.Join(root, ".eve-fleet"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".eve-fleet", "audit.jsonl"), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "audit", "--outcome=out-1", "--json")
	if res.Code != 0 {
		t.Fatalf("audit: %s%s", res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, `"topologyVersion"`) || !strings.Contains(res.Stdout, `"gitSHA"`) {
		t.Fatalf("missing topology/git: %s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, `"payloadHash"`) {
		t.Fatal("payloadHash required")
	}
	if strings.Contains(res.Stdout, "SECRET_PAYLOAD") && strings.Contains(res.Stdout, `"payload"`) {
		t.Fatalf("must not echo payload: %s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "parent owns the outcome") {
		t.Fatalf("retrospective note missing: %s", res.Stdout)
	}
}
