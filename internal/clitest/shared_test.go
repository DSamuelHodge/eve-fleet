package clitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedAddRegistersFleetfileAndDoctorGreen(t *testing.T) {
	root := initFleet(t, "ops")
	if res := Run(t, root, nil, "agent", "add", "lead-intake",
		"--role=parent", "--outcome=o", "--sla=s", "--completion=c",
		"--approver=human", "--approval-timeout=15m"); res.Code != 0 {
		t.Fatalf("agent: %s%s", res.Stdout, res.Stderr)
	}
	for _, args := range [][]string{
		{"shared", "add", "skill", "lead-context"},
		{"shared", "add", "tool", "crm-write"},
		{"shared", "add", "connection", "hubspot"},
	} {
		res := Run(t, root, nil, args...)
		if res.Code != 0 {
			t.Fatalf("%v: %s%s", args, res.Stdout, res.Stderr)
		}
	}
	body, _ := os.ReadFile(filepath.Join(root, "Fleetfile"))
	text := string(body)
	for _, want := range []string{"lead-context", "crm-write", "hubspot", "approver: human"} {
		if !strings.Contains(text, want) {
			t.Errorf("Fleetfile missing %q\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "shared/tools/crm-write")); err != nil {
		t.Fatal(err)
	}
	doc := Run(t, root, nil, "doctor", "--json")
	if doc.Code != 0 {
		t.Fatalf("doctor: %s%s", doc.Stdout, doc.Stderr)
	}
	if !strings.Contains(doc.Stdout, "approval.human") {
		t.Fatalf("expected human note: %s", doc.Stdout)
	}
	if !strings.Contains(doc.Stdout, `"ok": true`) && !strings.Contains(doc.Stdout, `"ok":true`) {
		t.Fatalf("human must be non-blocking: %s", doc.Stdout)
	}
}

func TestSharedAddJSON(t *testing.T) {
	root := initFleet(t, "json-ops")
	res := Run(t, root, nil, "shared", "add", "tool", "crm-write", "--json")
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
	if !payload.OK || payload.Name != "crm-write" {
		t.Fatalf("%+v", payload)
	}
}

func TestDoctorRejectsUnknownApprover(t *testing.T) {
	root := initFleet(t, "bad-appr")
	if res := Run(t, root, nil, "agent", "add", "lead-intake",
		"--role=parent", "--outcome=o", "--sla=s", "--completion=c",
		"--approver=ghost"); res.Code == 0 {
		t.Fatal("expected unknown approver to fail")
	}
}

func TestDoctorRejectsSecretsInFleetfile(t *testing.T) {
	root := initFleet(t, "secrets")
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	mut := string(body) + "\nextra:\n  api_key: hunter2\n"
	if err := os.WriteFile(ff, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code == 0 {
		t.Fatal("expected secret rejection")
	}
	if !strings.Contains(res.Stdout, "fleetfile.secret") {
		t.Fatalf("got %s", res.Stdout)
	}
}

func TestSharedAddRejectsBadKind(t *testing.T) {
	root := initFleet(t, "kind")
	res := Run(t, root, nil, "shared", "add", "widget", "x")
	if res.Code == 0 {
		t.Fatal("expected bad kind")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "shared.kind") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}
