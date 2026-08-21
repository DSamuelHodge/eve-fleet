package fleetfile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "eve.fleet/v1"
	Kind       = "Fleet"
	FileName   = "Fleetfile"
	MaxAgents  = 50
)

var (
	dnsLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	semver   = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	gitSHA   = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
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
	Path  string `yaml:"path"`
	Role  string `yaml:"role"`
	Owns  Owns   `yaml:"owns"`
	Model string `yaml:"model,omitempty"`
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

func Validate(doc *Document, root string) []diag.Diagnostic {
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
	if doc.Metadata.Version == "" || !semver.MatchString(strings.TrimSpace(doc.Metadata.Version)) {
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
	}
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
		ds = append(ds, validateGit(root)...)
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

func validateGit(root string) []diag.Diagnostic {
	if !gitOK(root, "rev-parse", "--is-inside-work-tree") {
		return []diag.Diagnostic{diag.Error(".", "runtime.git.required",
			"fleet must be a git repository",
			"run eve-fleet init <name> or git init")}
	}
	sha, err := RevParse(root)
	if err != nil || !gitSHA.MatchString(sha) {
		return []diag.Diagnostic{diag.Error(".", "runtime.git.pin",
			"git HEAD must be a real commit SHA",
			"commit the Fleetfile so revision is pinned")}
	}
	return nil
}

func RevParse(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitOK(root string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd.Run() == nil
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
