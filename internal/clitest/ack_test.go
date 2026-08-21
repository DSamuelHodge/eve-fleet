package clitest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequiresAckGeneratesAckRejectOnParent(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=route_lead",
		"--from=lead-intake",
		"--to=dedupe",
		"--contract=scored_lead -> route_result",
		"--requires-ack",
		"--timeout=15m",
		"--on-failure=retry")
	if res.Code != 0 {
		t.Fatalf("edge add: %s%s", res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "ack/reject") {
		t.Fatalf("expected ack mention: %s", res.Stdout)
	}
	dir := filepath.Join(root, "agents/lead-intake/agent/tools/fleet")
	ack, err := os.ReadFile(filepath.Join(dir, "ack_edge_route_lead.ts"))
	if err != nil {
		t.Fatal(err)
	}
	rej, err := os.ReadFile(filepath.Join(dir, "reject_edge_route_lead.ts"))
	if err != nil {
		t.Fatal(err)
	}
	as, rs := string(ack), string(rej)
	for _, want := range []string{"defineTool", "reason", "Callable only after edge_route_lead has completed"} {
		if !strings.Contains(as, want) {
			t.Errorf("ack missing %q\n%s", want, as)
		}
	}
	if !strings.Contains(rs, "Reject fails the parent outcome") {
		t.Errorf("reject missing outcome fail:\n%s", rs)
	}
	if !strings.Contains(as, "Timeout without ack/reject fails the edge and the parent outcome") {
		t.Errorf("ack missing timeout semantics:\n%s", as)
	}
	calleeAck := filepath.Join(root, "agents/dedupe/agent/tools/fleet/ack_edge_route_lead.ts")
	if _, err := os.Stat(calleeAck); err == nil {
		t.Fatal("ack tools must not be generated on the callee")
	}
	ff, _ := os.ReadFile(filepath.Join(root, "Fleetfile"))
	if !strings.Contains(string(ff), "requires_ack: true") {
		t.Fatalf("Fleetfile missing requires_ack:\n%s", ff)
	}
	doc := Run(t, root, nil, "doctor", "--json")
	if doc.Code != 0 {
		t.Fatalf("doctor: %s%s", doc.Stdout, doc.Stderr)
	}
	if !strings.Contains(doc.Stdout, "edge.retry.degraded") {
		t.Fatalf("expected retry degradation note: %s", doc.Stdout)
	}
}

func TestRequiresAckRejectedFromDelegate(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=back", "--from=dedupe", "--to=lead-intake",
		"--contract=c", "--requires-ack")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "edge.requires_ack.parent") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}

func TestOnFailureValues(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=x", "--from=lead-intake", "--to=dedupe",
		"--contract=c", "--on-failure=explode")
	if res.Code == 0 {
		t.Fatal("expected bad on_failure")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "edge.on_failure") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
	ok := Run(t, root, nil, "edge", "add",
		"--name=fail_edge", "--from=lead-intake", "--to=dedupe",
		"--contract=c", "--on-failure=fail")
	if ok.Code != 0 {
		t.Fatal(ok.Stderr)
	}
}

func TestDefaultAccountabilityIsRetrospective(t *testing.T) {
	root := seededFleet(t)
	if res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead", "--from=lead-intake", "--to=dedupe", "--contract=c"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	ff, _ := os.ReadFile(filepath.Join(root, "Fleetfile"))
	if strings.Contains(string(ff), "requires_ack: true") {
		t.Fatal("default must not set requires_ack")
	}
	ack := filepath.Join(root, "agents/lead-intake/agent/tools/fleet/ack_edge_dedupe_lead.ts")
	if _, err := os.Stat(ack); err == nil {
		t.Fatal("retrospective edges must not generate ack tools")
	}
}
