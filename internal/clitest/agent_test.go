package clitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentAddParentAndDelegateDoctorGreen(t *testing.T) {
	root := initFleet(t, "ops")
	parent := Run(t, root, nil, "agent", "add", "lead-intake",
		"--role=parent",
		"--outcome=Inbound lead reaches terminal state",
		"--sla=P95 < 15m",
		"--completion=qualified or rejected",
		"--yes", "--non-interactive")
	if parent.Code != 0 {
		t.Fatalf("parent add: %s%s", parent.Stdout, parent.Stderr)
	}
	del := Run(t, root, nil, "agent", "add", "dedupe",
		"--role=delegate",
		"--job=Deduplicate against CRM",
		"--contract=raw_lead -> unique_or_match")
	if del.Code != 0 {
		t.Fatalf("delegate add: %s%s", del.Stdout, del.Stderr)
	}

	ff, err := os.ReadFile(filepath.Join(root, "Fleetfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(ff)
	for _, want := range []string{
		"lead-intake:",
		"role: parent",
		"path: agents/lead-intake",
		"dedupe:",
		"role: delegate",
		"path: agents/dedupe",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Fleetfile missing %q\n%s", want, text)
		}
	}

	parentInstr, _ := os.ReadFile(filepath.Join(root, "agents/lead-intake/agent/instructions.md"))
	ps := string(parentInstr)
	for _, want := range []string{"owns.outcome", "owns.sla", "owns.completion"} {
		if !strings.Contains(ps, want) {
			t.Errorf("parent instructions missing %s\n%s", want, ps)
		}
	}
	delInstr, _ := os.ReadFile(filepath.Join(root, "agents/dedupe/agent/instructions.md"))
	ds := string(delInstr)
	for _, want := range []string{"owns.job", "owns.contract"} {
		if !strings.Contains(ds, want) {
			t.Errorf("delegate instructions missing %s\n%s", want, ds)
		}
	}
	for _, p := range []string{
		"agents/lead-intake/agent/agent.ts",
		"agents/dedupe/agent/agent.ts",
		"agents/lead-intake/agent/tools/fleet",
		"agents/dedupe/agent/subagents",
	} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}

	doc := Run(t, root, nil, "doctor")
	if doc.Code != 0 {
		t.Fatalf("doctor: %s%s", doc.Stdout, doc.Stderr)
	}
}

func TestAgentAddRequiresRoleAndOwns(t *testing.T) {
	root := initFleet(t, "needs")
	res := Run(t, root, nil, "agent", "add", "lead-intake", "--non-interactive")
	if res.Code == 0 {
		t.Fatal("expected failure without --role")
	}
	out := res.Stdout + res.Stderr
	if !strings.Contains(out, "agent.role") || !strings.Contains(out, "suggestion:") {
		t.Fatalf("expected role diagnostic:\n%s", out)
	}
}

func TestDoctorRejectsRoleMixAndBadPath(t *testing.T) {
	root := initFleet(t, "mix")
	if res := Run(t, root, nil, "agent", "add", "lead-intake",
		"--role=parent", "--outcome=o", "--sla=s", "--completion=c"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	mut := strings.Replace(string(body), "path: agents/lead-intake", "path: agents/wrong", 1)
	mut = strings.Replace(mut, "completion: c", "completion: c\n      job: not-for-parent", 1)
	if err := os.WriteFile(ff, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code == 0 {
		t.Fatal("expected doctor failure")
	}
	if !strings.Contains(res.Stdout, "agent.path") {
		t.Errorf("missing agent.path:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "agent.owns.role") {
		t.Errorf("missing owns.role:\n%s", res.Stdout)
	}
}

func TestDoctorRejectsDelegateWithParentOwns(t *testing.T) {
	root := initFleet(t, "del-mix")
	if res := Run(t, root, nil, "agent", "add", "dedupe",
		"--role=delegate", "--job=j", "--contract=c"); res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	ff := filepath.Join(root, "Fleetfile")
	body, _ := os.ReadFile(ff)
	mut := strings.Replace(string(body), "contract: c", "contract: c\n      outcome: not-for-delegate", 1)
	if err := os.WriteFile(ff, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Run(t, root, nil, "doctor", "--json")
	if res.Code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Stdout, "agent.owns.role") {
		t.Fatalf("got %s", res.Stdout)
	}
}

func TestDoctorRejectsFiftyFirstAgentUnderFiveSeconds(t *testing.T) {
	root := initFleet(t, "scale")
	ff := filepath.Join(root, "Fleetfile")
	var b strings.Builder
	b.WriteString("apiVersion: eve.fleet/v1\nkind: Fleet\nmetadata:\n  name: scale\n  version: \"0.1.0\"\nagents:\n")
	for i := 0; i < 50; i++ {
		name := "agent-" + two(i)
		b.WriteString("  " + name + ":\n")
		b.WriteString("    path: agents/" + name + "\n")
		b.WriteString("    role: parent\n")
		b.WriteString("    owns:\n      outcome: o\n      sla: s\n      completion: c\n")
	}
	b.WriteString("edges: []\nruntime:\n  isolation: strong\n  supervisor: false\n  hot_load:\n    agents: true\n    shared: true\n  git:\n    required: true\n    pin: commit\n")
	if err := os.WriteFile(ff, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res := Run(t, root, nil, "doctor", "--json")
	elapsed := time.Since(start)
	if res.Code != 0 {
		t.Fatalf("50-agent doctor should pass: %s%s", res.Stdout, res.Stderr)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("doctor on 50 agents took %s", elapsed)
	}
	add := Run(t, root, nil, "agent", "add", "agent-50",
		"--role=parent", "--outcome=o", "--sla=s", "--completion=c")
	if add.Code == 0 {
		t.Fatal("51st agent should fail")
	}
	if !strings.Contains(add.Stdout+add.Stderr, "fleetfile.agents.limit") {
		t.Fatalf("expected limit rule:\n%s%s", add.Stdout, add.Stderr)
	}

	var over strings.Builder
	over.WriteString("apiVersion: eve.fleet/v1\nkind: Fleet\nmetadata:\n  name: scale\n  version: \"0.1.0\"\nagents:\n")
	for i := 0; i < 51; i++ {
		name := "agent-" + two(i)
		over.WriteString("  " + name + ":\n")
		over.WriteString("    path: agents/" + name + "\n")
		over.WriteString("    role: parent\n")
		over.WriteString("    owns:\n      outcome: o\n      sla: s\n      completion: c\n")
	}
	if err := os.WriteFile(ff, []byte(over.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	doc51 := Run(t, root, nil, "doctor", "--json")
	if doc51.Code == 0 {
		t.Fatal("doctor should reject 51 agents")
	}
	if !strings.Contains(doc51.Stdout, "fleetfile.agents.limit") {
		t.Fatalf("expected limit on doctor:\n%s", doc51.Stdout)
	}
}

func TestAgentAddJSON(t *testing.T) {
	root := initFleet(t, "json-ops")
	res := Run(t, root, nil, "agent", "add", "lead-intake",
		"--role=parent", "--outcome=o", "--sla=s", "--completion=c", "--json")
	if res.Code != 0 {
		t.Fatal(res.Stderr)
	}
	var payload struct {
		OK   bool   `json:"ok"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		t.Fatalf("%v\n%s", err, res.Stdout)
	}
	if !payload.OK || payload.Name != "lead-intake" {
		t.Fatalf("%+v", payload)
	}
}

func two(i int) string {
	return string('0'+byte(i/10)) + string('0'+byte(i%10))
}
