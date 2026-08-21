package fleetfile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "eve.fleet/v1"
	Kind       = "Fleet"
	FileName   = "Fleetfile"
	MaxAgents  = 50
	GitTimeout = 30 * time.Second
)

var (
	dnsLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	// SemVer 2.0.0 core + optional pre-release and build metadata.
	semver   = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	gitSHA   = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	edgeName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

type Document struct {
	APIVersion string               `yaml:"apiVersion"`
	Kind       string               `yaml:"kind"`
	Metadata   Metadata             `yaml:"metadata"`
	Shared     *Shared              `yaml:"shared,omitempty"`
	Agents     map[string]AgentSpec `yaml:"agents"`
	Edges      []EdgeSpec           `yaml:"edges"`
	Runtime    *Runtime             `yaml:"runtime,omitempty"`
}

type Metadata struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description,omitempty"`
	Owners      []string          `yaml:"owners,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

type Shared struct {
	Skills      []string `yaml:"skills,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
	Connections []string `yaml:"connections,omitempty"`
}

type AgentSpec struct {
	Path           string          `yaml:"path"`
	Role           string          `yaml:"role"`
	Owns           Owns            `yaml:"owns"`
	ApprovalPolicy *ApprovalPolicy `yaml:"approval_policy,omitempty"`
	Model          string          `yaml:"model,omitempty"`
	Description    string          `yaml:"description,omitempty"`
}

type ApprovalPolicy struct {
	Approver string                  `yaml:"approver,omitempty"`
	Timeout  string                  `yaml:"timeout,omitempty"`
	Tools    map[string]ToolApproval `yaml:"tools,omitempty"`
}

type ToolApproval struct {
	Approver string `yaml:"approver,omitempty"`
	Timeout  string `yaml:"timeout,omitempty"`
}

type Owns struct {
	Outcome    string `yaml:"outcome,omitempty"`
	SLA        string `yaml:"sla,omitempty"`
	Completion string `yaml:"completion,omitempty"`
	Job        string `yaml:"job,omitempty"`
	Contract   string `yaml:"contract,omitempty"`
}

type EdgeSpec struct {
	Name        string `yaml:"name"`
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	Contract    string `yaml:"contract"`
	Timeout     string `yaml:"timeout,omitempty"`
	OnFailure   string `yaml:"on_failure,omitempty"`
	RequiresAck bool   `yaml:"requires_ack,omitempty"`
}

type Runtime struct {
	Isolation  string  `yaml:"isolation,omitempty"`
	Supervisor bool    `yaml:"supervisor"`
	HotLoad    HotLoad `yaml:"hot_load"`
	Git        Git     `yaml:"git"`
}

type HotLoad struct {
	Agents *bool `yaml:"agents,omitempty"`
	Shared *bool `yaml:"shared,omitempty"`
}

type Git struct {
	Required *bool  `yaml:"required,omitempty"`
	Pin      string `yaml:"pin,omitempty"`
}

func Defaults() Runtime {
	agents, shared, required := true, true, true
	return Runtime{
		Isolation:  "strong",
		Supervisor: false,
		HotLoad:    HotLoad{Agents: &agents, Shared: &shared},
		Git:        Git{Required: &required, Pin: "commit"},
	}
}

func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func Load(root string) (*Document, []byte, error) {
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		return nil, nil, err
	}
	doc, err := Parse(data)
	return doc, data, err
}

func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func ValidDNSLabel(name string) bool {
	if len(name) < 1 || len(name) > 63 {
		return false
	}
	return dnsLabel.MatchString(name)
}

func Validate(ctx context.Context, doc *Document, root string, raw []byte) []diag.Diagnostic {
	var ds []diag.Diagnostic
	if doc.APIVersion != APIVersion {
		ds = append(ds, diag.Error("Fleetfile", "fleetfile.apiVersion",
			fmt.Sprintf("apiVersion must be %s", APIVersion),
			fmt.Sprintf("set apiVersion: %s", APIVersion)))
	}
	if doc.Kind != Kind {
		ds = append(ds, diag.Error("Fleetfile", "fleetfile.kind",
			"kind must be Fleet",
			"set kind: Fleet"))
	}
	if !ValidDNSLabel(doc.Metadata.Name) {
		ds = append(ds, diag.Error("metadata.name", "metadata.name",
			"metadata.name must be a DNS-label (lowercase letters, digits, hyphens)",
			"use a name like revenue-ops"))
	}
	if doc.Metadata.Version == "" || !semver.MatchString(doc.Metadata.Version) {
		ds = append(ds, diag.Error("metadata.version", "metadata.version",
			"metadata.version must be human-managed semver",
			`set metadata.version: "0.1.0"`))
	}
	if doc.Agents == nil {
		ds = append(ds, diag.Error("agents", "fleetfile.agents.required",
			"agents map is required (empty {} is valid)",
			"add agents: {} or eve-fleet agent add <name> --role=parent"))
	} else if len(doc.Agents) > MaxAgents {
		ds = append(ds, diag.Error("agents", "fleetfile.agents.limit",
			fmt.Sprintf("at most %d agents", MaxAgents),
			"remove agents until the fleet has 50 or fewer"))
	} else {
		ds = append(ds, validateAgents(doc.Agents)...)
		ds = append(ds, validateApproval(doc.Agents)...)
	}
	ds = append(ds, ValidateEdges(doc)...)
	ds = append(ds, validateSecrets(raw)...)
	rt := doc.Runtime
	if rt == nil {
		d := Defaults()
		rt = &d
	}
	if rt.Isolation != "" && rt.Isolation != "strong" && rt.Isolation != "shared-sandbox-pool" {
		ds = append(ds, diag.Error("runtime.isolation", "runtime.isolation",
			`isolation must be "strong" or "shared-sandbox-pool"`,
			`set runtime.isolation: strong`))
	}
	gitRequired := true
	if rt.Git.Required != nil {
		gitRequired = *rt.Git.Required
	}
	if pin := rt.Git.Pin; pin != "" && pin != "commit" {
		ds = append(ds, diag.Error("runtime.git.pin", "runtime.git.pin",
			`git pin must be "commit"`,
			`set runtime.git.pin: commit`))
	}
	if gitRequired {
		ds = append(ds, validateGit(ctx, root)...)
	}
	if _, err := os.Stat(filepath.Join(root, "Fleet.lock")); err == nil {
		ds = append(ds, diag.Error("Fleet.lock", "fleet.lock.forbidden",
			"Fleet.lock is not part of v1; git SHA is the only lock",
			"delete Fleet.lock"))
	}
	if _, err := os.Stat(filepath.Join(root, "VERSION")); err == nil {
		ds = append(ds, diag.Error("VERSION", "version.file.forbidden",
			"root VERSION file is not part of v1; git SHA is the only lock",
			"delete VERSION"))
	}
	return ds
}

func validateGit(ctx context.Context, root string) []diag.Diagnostic {
	if !insideWorkTree(ctx, root) {
		return []diag.Diagnostic{diag.Error(".", "runtime.git.required",
			"fleet must be a git work tree (bare repositories are not valid)",
			"run eve-fleet init <name> or git init")}
	}
	sha, err := RevParse(ctx, root)
	if err != nil || !gitSHA.MatchString(sha) {
		return []diag.Diagnostic{diag.Error(".", "runtime.git.pin",
			"git HEAD must be a real commit SHA",
			"commit the Fleetfile so revision is pinned")}
	}
	return nil
}

func RevParse(ctx context.Context, root string) (string, error) {
	cmd := gitCommand(ctx, root, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func insideWorkTree(ctx context.Context, root string) bool {
	out, err := gitCommand(ctx, root, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func gitCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func Save(root string, doc *Document) error {
	path := filepath.Join(root, FileName)
	f, err := os.CreateTemp(root, FileName+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if f != nil {
			_ = f.Close()
		}
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		f = nil
		return err
	}
	f = nil
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func AgentPath(name string) string {
	return "agents/" + name
}

func validateAgents(agents map[string]AgentSpec) []diag.Diagnostic {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	var ds []diag.Diagnostic
	for _, name := range names {
		ds = append(ds, ValidateSpec(name, agents[name])...)
	}
	return ds
}

func ValidateSpec(name string, spec AgentSpec) []diag.Diagnostic {
	var ds []diag.Diagnostic
	base := "agents." + name
	if !ValidDNSLabel(name) {
		ds = append(ds, diag.Error(base, "agent.name",
			"agent name must be a DNS-label (lowercase letters, digits, hyphens)",
			"rename the agent to a DNS-label"))
	}
	want := AgentPath(name)
	if spec.Path != want {
		ds = append(ds, diag.Error(base+".path", "agent.path",
			fmt.Sprintf("path must be %s", want),
			fmt.Sprintf("set path: %s", want)))
	}
	switch spec.Role {
	case "parent":
		if spec.Owns.Outcome == "" || spec.Owns.SLA == "" || spec.Owns.Completion == "" {
			ds = append(ds, diag.Error(base+".owns", "agent.owns.parent",
				"parent requires owns.outcome, owns.sla, and owns.completion",
				"set outcome, sla, and completion; do not set job or contract"))
		}
		if spec.Owns.Job != "" || spec.Owns.Contract != "" {
			ds = append(ds, diag.Error(base+".owns", "agent.owns.role",
				"parent must not declare owns.job or owns.contract",
				"remove job/contract from the parent"))
		}
	case "delegate":
		if spec.Owns.Job == "" || spec.Owns.Contract == "" {
			ds = append(ds, diag.Error(base+".owns", "agent.owns.delegate",
				"delegate requires owns.job and owns.contract",
				"set job and contract; do not set outcome, sla, or completion"))
		}
		if spec.Owns.Outcome != "" || spec.Owns.SLA != "" || spec.Owns.Completion != "" {
			ds = append(ds, diag.Error(base+".owns", "agent.owns.role",
				"delegate must not declare owns.outcome, owns.sla, or owns.completion",
				"remove outcome/sla/completion from the delegate"))
		}
	default:
		ds = append(ds, diag.Error(base+".role", "agent.role",
			`role must be "parent" or "delegate"`,
			`set role: parent or role: delegate`))
	}
	return ds
}

func ValidEdgeName(name string) bool {
	return len(name) > 0 && len(name) <= 63 && edgeName.MatchString(name)
}

func ValidateEdges(doc *Document) []diag.Diagnostic {
	supervisor := false
	if doc != nil && doc.Runtime != nil {
		supervisor = doc.Runtime.Supervisor
	}
	var agents map[string]AgentSpec
	var edges []EdgeSpec
	if doc != nil {
		agents, edges = doc.Agents, doc.Edges
	}
	return validateEdges(agents, edges, supervisor)
}

func validateEdges(agents map[string]AgentSpec, edges []EdgeSpec, supervisor bool) []diag.Diagnostic {
	var ds []diag.Diagnostic
	seen := map[string]int{}
	for i, e := range edges {
		base := fmt.Sprintf("edges[%d]", i)
		if !ValidEdgeName(e.Name) {
			ds = append(ds, diag.Error(base+".name", "edge.name",
				"edge name must be lowercase letters, digits, and underscores",
				"use a name like dedupe_lead"))
		} else if prev, ok := seen[e.Name]; ok {
			ds = append(ds, diag.Error(base+".name", "edge.name.unique",
				fmt.Sprintf("edge name %q already used at edges[%d]", e.Name, prev),
				"give each edge a unique name"))
		} else {
			seen[e.Name] = i
		}
		if strings.TrimSpace(e.Contract) == "" {
			ds = append(ds, diag.Error(base+".contract", "edge.contract",
				"edge contract is required free-text",
				"set a contract describing the handoff"))
		}
		if !agentExists(agents, e.From) {
			ds = append(ds, diag.Error(base+".from", "edge.endpoint",
				fmt.Sprintf("from %q does not name an existing agent", e.From),
				"set from to an agent declared in agents:"))
		}
		if !agentExists(agents, e.To) {
			ds = append(ds, diag.Error(base+".to", "edge.endpoint",
				fmt.Sprintf("to %q does not name an existing agent", e.To),
				"set to to an agent declared in agents:"))
		}
		switch e.OnFailure {
		case "", "parent_handles", "retry", "escalate", "fail":
		default:
			ds = append(ds, diag.Error(base+".on_failure", "edge.on_failure",
				`on_failure must be parent_handles, retry, escalate, or fail`,
				`set on_failure: parent_handles`))
		}
		if e.Timeout != "" {
			if _, err := time.ParseDuration(e.Timeout); err != nil {
				ds = append(ds, diag.Error(base+".timeout", "edge.timeout",
					"timeout must be a Go duration such as 15m or 1h",
					`set timeout: "15m"`))
			}
		}
		if e.RequiresAck {
			if spec, ok := agents[e.From]; !ok || spec.Role != "parent" {
				ds = append(ds, diag.Error(base+".requires_ack", "edge.requires_ack.parent",
					"requires_ack is only valid on edges from a parent",
					"set from to a parent agent or disable requires_ack"))
			}
		}
		if e.OnFailure == "retry" && !supervisor {
			ds = append(ds, diag.Note(base+".on_failure", "edge.retry.degraded",
				"retry without a supervisor degrades to parent_handles",
				"set runtime.supervisor: true for supervisor-backed retry"))
		}
	}
	if cycle := findCycle(edges, agents); len(cycle) > 0 {
		ds = append(ds, diag.Error("edges", "edge.cycle",
			"edges form a cycle: "+strings.Join(cycle, " -> "),
			"remove or reverse an edge so the graph is a DAG"))
	}
	return ds
}

func findCycle(edges []EdgeSpec, agents map[string]AgentSpec) []string {
	adj := map[string][]string{}
	for _, e := range edges {
		if !agentExists(agents, e.From) || !agentExists(agents, e.To) {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var cycle []string
	var dfs func(string) bool
	dfs = func(u string) bool {
		color[u] = gray
		stack = append(stack, u)
		for _, v := range adj[u] {
			switch color[v] {
			case white:
				if dfs(v) {
					return true
				}
			case gray:
				i := 0
				for i < len(stack) && stack[i] != v {
					i++
				}
				cycle = append(append([]string{}, stack[i:]...), v)
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = black
		return false
	}
	names := make([]string, 0, len(agents))
	for n := range agents {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if color[n] == white {
			if dfs(n) {
				return cycle
			}
		}
	}
	return nil
}

func ValidateApproval(agents map[string]AgentSpec) []diag.Diagnostic {
	return validateApproval(agents)
}

func validateApproval(agents map[string]AgentSpec) []diag.Diagnostic {
	var ds []diag.Diagnostic
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := agents[name]
		if spec.ApprovalPolicy == nil {
			continue
		}
		ds = append(ds, checkApprover("agents."+name+".approval_policy", spec.ApprovalPolicy.Approver, spec.ApprovalPolicy.Timeout, agents)...)
		toolNames := make([]string, 0, len(spec.ApprovalPolicy.Tools))
		for tn := range spec.ApprovalPolicy.Tools {
			toolNames = append(toolNames, tn)
		}
		sort.Strings(toolNames)
		for _, tn := range toolNames {
			ta := spec.ApprovalPolicy.Tools[tn]
			ds = append(ds, checkApprover("agents."+name+".approval_policy.tools."+tn, ta.Approver, ta.Timeout, agents)...)
		}
	}
	return ds
}

func checkApprover(path, approver, timeout string, agents map[string]AgentSpec) []diag.Diagnostic {
	var ds []diag.Diagnostic
	if approver == "" {
		ds = append(ds, diag.Error(path+".approver", "approval.approver",
			"approver is required when approval_policy is set",
			`set approver to an agent name or "human"`))
	} else if approver != "human" && !agentExists(agents, approver) {
		ds = append(ds, diag.Error(path+".approver", "approval.approver",
			fmt.Sprintf("approver %q is not an agent in this fleet or \"human\"", approver),
			`set approver to an existing agent name or "human"`))
	} else if approver == "human" {
		ds = append(ds, diag.Note(path+".approver", "approval.human",
			`"human" is a non-blocking v1 approver (audit note only)`,
			"keep using a named agent if you need a blocking approver"))
	}
	if timeout != "" {
		if _, err := time.ParseDuration(timeout); err != nil {
			ds = append(ds, diag.Error(path+".timeout", "approval.timeout",
				"timeout must be a Go duration such as 15m or 1h",
				`set timeout: "15m"`))
		}
	}
	return ds
}

var secretKey = regexp.MustCompile(`(?i)(?:^|[_-])(password|passwd|secret|token|api[_-]?key|private[_-]?key|credential|credentials|auth)(?:$|[_-])`)

func validateSecrets(raw []byte) []diag.Diagnostic {
	if len(raw) == 0 {
		return []diag.Diagnostic{diag.Error("Fleetfile", "fleetfile.secret.scan",
			"Fleetfile bytes unavailable for secret scan",
			"re-run doctor after saving the Fleetfile")}
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return []diag.Diagnostic{diag.Error("Fleetfile", "fleetfile.parse",
			err.Error(),
			"fix YAML syntax in Fleetfile")}
	}
	var ds []diag.Diagnostic
	walkSecrets(&node, "", &ds)
	return ds
}

func walkSecrets(n *yaml.Node, path string, ds *[]diag.Diagnostic) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walkSecrets(c, path, ds)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			key := k.Value
			child := key
			if path != "" {
				child = path + "." + key
			}
			if secretKey.MatchString(key) {
				*ds = append(*ds, diag.Error(child, "fleetfile.secret",
					"Fleetfile must not hold secrets",
					"move the secret to the environment or a connection store, not the Fleetfile"))
			}
			walkSecrets(v, child, ds)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			walkSecrets(c, fmt.Sprintf("%s[%d]", path, i), ds)
		}
	}
}

func agentExists(agents map[string]AgentSpec, name string) bool {
	if agents == nil || name == "" {
		return false
	}
	_, ok := agents[name]
	return ok
}

func ScaffoldYAML(name string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: eve.fleet/v1
kind: Fleet

metadata:
  name: %s
  version: "0.1.0"

agents: {}
edges: []

runtime:
  isolation: strong
  supervisor: false
  hot_load:
    agents: true
    shared: true
  git:
    required: true
    pin: commit
`, name))
}
