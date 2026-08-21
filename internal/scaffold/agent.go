package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
)

var ErrTreeExists = errors.New("agent tree already exists")

func AgentTree(root string, name string, spec fleetfile.AgentSpec) error {
	base := filepath.Join(root, "agents", name, "agent")
	agentFile := filepath.Join(base, "agent.ts")
	if _, err := os.Stat(agentFile); err == nil {
		return fmt.Errorf("%w: %s", ErrTreeExists, filepath.Join("agents", name, "agent"))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dirs := []string{
		filepath.Join(base, "tools", "fleet"),
		filepath.Join(base, "skills"),
		filepath.Join(base, "subagents"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(agentFile, []byte(agentTS()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "instructions.md"), []byte(instructions(name, spec)), 0o644)
}

func agentTS() string {
	return `import { defineAgent } from "eve";

export default defineAgent({});
`
}

func instructions(name string, spec fleetfile.AgentSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nRole: %s\n\n## Ownership\n\n", name, spec.Role)
	switch spec.Role {
	case "parent":
		fmt.Fprintf(&b, "- owns.outcome: %s\n", spec.Owns.Outcome)
		fmt.Fprintf(&b, "- owns.sla: %s\n", spec.Owns.SLA)
		fmt.Fprintf(&b, "- owns.completion:\n%s\n", indent(spec.Owns.Completion))
	case "delegate":
		fmt.Fprintf(&b, "- owns.job: %s\n", spec.Owns.Job)
		fmt.Fprintf(&b, "- owns.contract:\n%s\n", indent(spec.Owns.Contract))
	}
	return b.String()
}

func indent(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n") + "\n"
}
