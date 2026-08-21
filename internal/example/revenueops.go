package example

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/DSamuelHodge/eve-fleet/internal/scaffold"
	"gopkg.in/yaml.v3"
)

const RevenueOpsYAML = `apiVersion: eve.fleet/v1
kind: Fleet

metadata:
  name: revenue-ops
  version: "1.4.2"
  description: "Inbound lead qualification and routing"
  owners: ["@revops", "@platform"]

shared:
  skills: [revenue-definitions]
  tools: [crm_read]
  connections: [salesforce]

agents:
  lead-intake:
    path: agents/lead-intake
    role: parent
    owns:
      outcome: "Inbound lead reaches qualified-or-rejected terminal state"
      sla: "P95 end-to-end < 15 minutes"
      completion: "Lead scored and routed or explicitly rejected"
    approval_policy:
      approver: lead-intake
      timeout: 2h
      tools:
        finalize_outcome:
          approver: human
          timeout: 4h
  dedupe:
    path: agents/dedupe
    role: delegate
    owns:
      job: "Exact + fuzzy deduplication against CRM"
      contract: "Input: raw lead. Output: unique + match_id"
  enrich:
    path: agents/enrich
    role: delegate
    owns:
      job: "Enrich with firmographic and contact data"
      contract: "Input: lead. Output: enriched object"
  score:
    path: agents/score
    role: delegate
    owns:
      job: "Score lead against revenue rules and emit reasons"
      contract: "Input: enriched. Output: score + qualified"
  route:
    path: agents/route
    role: delegate
    owns:
      job: "Route qualified lead to SDR queue or emit rejection"
      contract: "Input: scored lead. Output: routed or rejected"
    approval_policy:
      approver: route
      tools:
        write_sdr_queue:
          approver: human

edges:
  - name: dedupe_lead
    from: lead-intake
    to: dedupe
    contract: "raw_lead → dedupe_result"
    timeout: 20s
    on_failure: parent_handles
  - name: enrich_lead
    from: lead-intake
    to: enrich
    contract: "deduped_lead → enriched_lead"
    timeout: 45s
    on_failure: parent_handles
  - name: score_lead
    from: lead-intake
    to: score
    contract: "enriched_lead → score_result"
    timeout: 15s
    on_failure: parent_handles
  - name: route_lead
    from: lead-intake
    to: route
    contract: "scored_lead → route_result"
    timeout: 30s
    on_failure: fail
    requires_ack: true

runtime:
  isolation: strong
  supervisor: true
  hot_load:
    agents: true
    shared: true
  git:
    required: true
    pin: commit
`

func RevenueOps(root string) error {
	if err := os.WriteFile(filepath.Join(root, fleetfile.FileName), []byte(RevenueOpsYAML), 0o644); err != nil {
		return err
	}
	var doc fleetfile.Document
	if err := yaml.Unmarshal([]byte(RevenueOpsYAML), &doc); err != nil {
		return err
	}
	for name, spec := range doc.Agents {
		if err := scaffold.AgentTree(root, name, spec); err != nil {
			return fmt.Errorf("agent %s: %w", name, err)
		}
	}
	for _, edge := range doc.Edges {
		if err := scaffold.EdgeTool(root, edge); err != nil {
			return fmt.Errorf("edge %s: %w", edge.Name, err)
		}
	}
	if doc.Shared != nil {
		for _, n := range doc.Shared.Skills {
			_ = os.MkdirAll(filepath.Join(root, "shared", "skills", n), 0o755)
		}
		for _, n := range doc.Shared.Tools {
			_ = os.MkdirAll(filepath.Join(root, "shared", "tools", n), 0o755)
		}
		for _, n := range doc.Shared.Connections {
			_ = os.MkdirAll(filepath.Join(root, "shared", "connections", n), 0o755)
		}
	}
	return nil
}
