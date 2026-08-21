package clitest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildIsByteStableAndPinsSHA(t *testing.T) {
	root := seededFleet(t)
	if res := Run(t, root, nil, "edge", "add",
		"--name=dedupe_lead", "--from=lead-intake", "--to=dedupe", "--contract=c"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	commitAll(t, root, "add edge")
	first := Run(t, root, nil, "build", "--json")
	if first.Code != 0 {
		t.Fatalf("build: %s%s", first.Stdout, first.Stderr)
	}
	var payload struct {
		OK     bool   `json:"ok"`
		GitSHA string `json:"gitSHA"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal([]byte(first.Stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.GitSHA == "" {
		t.Fatalf("%+v", payload)
	}
	man := filepath.Join(root, ".eve-fleet", "build", payload.GitSHA, "manifest.json")
	a, err := os.ReadFile(man)
	if err != nil {
		t.Fatal(err)
	}
	second := Run(t, root, nil, "build")
	if second.Code != 0 {
		t.Fatal(second.Stderr)
	}
	b, err := os.ReadFile(man)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("builds of the same SHA differed\n%s\n%s", a, b)
	}
	if _, err := os.Stat(filepath.Join(root, "Fleet.lock")); err == nil {
		t.Fatal("Fleet.lock must not exist")
	}
	if _, err := os.Stat(filepath.Join(root, "VERSION")); err == nil {
		t.Fatal("VERSION must not exist")
	}
}

func TestLinkAndDeployRecordRevision(t *testing.T) {
	root := seededFleet(t)
	commitAll(t, root, "seed agents")
	link := Run(t, root, nil, "link", "--json")
	if link.Code != 0 {
		t.Fatalf("link: %s%s", link.Stdout, link.Stderr)
	}
	dep := Run(t, root, nil, "deploy", "--json")
	if dep.Code != 0 {
		t.Fatalf("deploy: %s%s", dep.Stdout, dep.Stderr)
	}
	body, err := os.ReadFile(filepath.Join(root, ".eve-fleet", "deploy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Kind   string `json:"kind"`
		GitSHA string `json:"gitSHA"`
	}
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Kind != "deploy" || rec.GitSHA == "" {
		t.Fatalf("%+v %s", rec, body)
	}
	bad := Run(t, root, nil, "deploy", "--revision=deadbeef")
	if bad.Code == 0 {
		t.Fatal("expected revision mismatch")
	}
	if !strings.Contains(bad.Stdout+bad.Stderr, "deploy.revision") {
		t.Fatalf("got %s%s", bad.Stdout, bad.Stderr)
	}
}

func TestBuildFailsWhenDoctorFails(t *testing.T) {
	dir := t.TempDir()
	res := Run(t, dir, nil, "build")
	if res.Code == 0 {
		t.Fatal("expected missing fleetfile")
	}
}

func TestBuildRejectsDirtyWorkTree(t *testing.T) {
	root := seededFleet(t)
	res := Run(t, root, nil, "build", "--json")
	if res.Code == 0 {
		t.Fatal("expected dirty work tree to fail")
	}
	if !strings.Contains(res.Stdout+res.Stderr, "deploy.dirty") {
		t.Fatalf("got %s%s", res.Stdout, res.Stderr)
	}
}
