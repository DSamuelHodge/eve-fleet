package clitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEdgeAddDoctorGreen(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead",
		"--from=lead-intake",
		"--to=dedupe",
		"--contract=raw_lead -> unique_or_match")
	if res.Code != 0 {
		t.Fatalf("edge add: %s%s", res.Stdout, res.Stderr)
	}
	tool := filepath.Join(root, "agents/lead-intake/agent/tools/fleet/edge_dedupe_lead.ts")
	body, err := os.ReadFile(tool)
	if err != nil {
		t.Fatal(err)
	}
	ts := string(body)
	for _, want := range []string{
		`from "eve/tools"`,
		`from "zod"`,
		"defineTool",
		"payload",
		"context",
		`status: "ok"`,
		"raw_lead -> unique_or_match",
		"dedupe_lead",
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("generated tool missing %q\n%s", want, ts)
		}
	}
	callee := filepath.Join(root, "agents/dedupe/agent/tools/fleet/edge_dedupe_lead.ts")
	if _, err := os.Stat(callee); err == nil {
		t.Fatal("edge tool must not be generated on the callee")
	}
	doc := Run(t, root, nil, "doctor")
	if doc.Code != 0 {
		t.Fatalf("doctor: %s%s", doc.Stdout, doc.Stderr)
	}
}

func TestEdgeAddJSON(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead", "--from=lead-intake", "--to=dedupe",
		"--contract=c", "--json")
	if res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	var payload struct {
		OK   bool   `json:"ok"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.Name != "dedupe_lead" {
		t.Fatalf("%+v", payload)
	}
}

func TestDoctorRejectsDuplicateEdgeName(t *testing.T) {
	root := seededFleet(t)
	args := []string{"edge", "add", "--name=dedupe_lead", "--from=lead-intake", "--to=dedupe", "--contract=c"}
	if res := Run(t, root, nil, args...); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	res := Run(t, root, nil, args...)
	if res.Code == 0 {
		t.Fatal("duplicate should fail")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "edge.name.unique") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}

func TestDoctorRejectsDanglingEndpoint(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=ghost", "--from=lead-intake", "--to=missing", "--contract=c")
	if res.Code == 0 {
		t.Fatal("expected dangling failure")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "edge.endpoint") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}

func TestDoctorRejectsCycle(t *testing.T) {
	root := seededFleet(t)
	if res := Run(t, root, nil, "edge", "add",
		"--name=forward", "--from=lead-intake", "--to=dedupe", "--contract=c"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	res := Run(t, root, nil, "edge", "add",
		"--name=back", "--from=dedupe", "--to=lead-intake", "--contract=c")
	if res.Code == 0 {
		t.Fatal("cycle should fail")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "edge.cycle") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}

func TestEdgeAddRequiresContract(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead", "--from=lead-intake", "--to=dedupe")
	if res.Code == 0 {
		t.Fatal("expected missing contract")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "edge.contract") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}

func seededFleet(t *testing.T) string {
	t.Helper()
	root := initFleet(t, "ops")
	if res := Run(t, root, nil, "agent", "add", "lead-intake",
		"--role=parent", "--outcome=o", "--sla=s", "--completion=c"); res.Code != 0 {
		t.Fatalf("parent: %s", res.Stderr)
	}
	if res := Run(t, root, nil, "agent", "add", "dedupe",
		"--role=delegate", "--job=j", "--contract=c"); res.Code != 0 {
		t.Fatalf("delegate: %s", res.Stderr)
	}
	return root
}
