# Software Requirements Specification (SRS)
# Eve-Rails — Convention Layer & CLI for Vercel Eve Agent Fleets

**Version:** 0.3.0  
**Status:** Design-complete / Ready for implementation  
**Date:** 2026-08-20  

**Design status:** Design tree closed. All system-shape decisions settled (Rounds 1–5). This document is the frozen model for realization.

**Core dependencies:** Vercel Eve (npm `eve`, filesystem-first agent framework), Go 1.22+, Cobra, Charm Bubble Tea + Lip Gloss (TTY UX only).

---

## 1. Introduction

### 1.1 Purpose

Eve-Rails is a Rails-inspired convention layer and command-line interface for defining, generating, operating, and auditing **fleets** of Vercel Eve agents.

Eve already provides a strong primitive: an agent is a directory of files. Eve-Rails adds the higher-order structure so that collections of agents express real organizational topology — outcome-owning parents, narrow specialists, explicit handoff contracts, actor-traced approvals, and git-versioned accountability.

### 1.2 Primary Job

**Make ownership, handoffs, and accountability first-class and git-versioned** so an organization can later answer:

> “Who owned this outcome, under which topology revision and git SHA, and which actors performed which steps?”

Reducing friction of spinning up and operating fleets is a **constraint** (“keep cognitive load low”), not the primary job. Ceremony required for an honest audit trail is acceptable; ceremony that exists only for convenience is not.

### 1.3 Scope

**In scope (v1):**

- Fleetfile (declarative topology + ownership manifest)
- Conventional directory layout
- Go + Cobra CLI (`eve-fleet`) with optional Charm-based TTY presentation
- Mapping onto pure Eve agent directories (no fork of Eve runtime)
- Git as source of truth
- Surgical hot-load of implementation trees only
- Actor-scoped approval model
- Optional supervisor for full runtime guarantees
- Integration with Eve subagents for intra-agent decomposition
- Fleet-level audit surface

**Out of scope (v1):**

- Replacing or forking the Eve runtime / compiler
- Full multi-tenant control plane / SaaS dashboard
- Automatic mass generation of agents from natural language
- Custom workflow engine beyond Eve Workflows
- Non-git version-control backends
- Multi-fleet / hierarchical fleets
- Rich approval policies (any-of, all-of, ordered chains)
- Blocking human-in-the-loop UI (schema accepts `"human"`; runtime is non-blocking)
- Separate lockfile (`Fleet.lock`)
- TUI-only operation (headless / CI paths must work fully)

### 1.4 Hard Constraints

| Constraint | Value |
|------------|-------|
| Max agents per fleet (v1) | 50 (performance + cognitive ceiling) |
| Cycles | Hard error |
| Topology changes on hot-load | Forbidden |
| Path convention | Strictly `agents/<name>` |
| Handoff authority | Edges are the single source of truth |
| Eve compatibility | Generated artifacts must be valid for stock Eve |
| CLI headless operation | Must function with zero TUI when not a TTY or when `--json` / `NO_COLOR` |

### 1.5 Definitions

| Term | Definition |
|------|------------|
| **Fleet** | Versioned collection of Eve agents + shared capabilities + explicit topology. |
| **Parent agent** | Owns an **outcome** (SLA + completion criteria). Accountable even when work is delegated. |
| **Delegate agent** | Owns a **narrow, checkable job** with a typed contract. |
| **Edge** | Named, typed handoff (from, to, contract, timeout, on_failure, requires_ack). Single source of truth for allowed handoffs. |
| **Subagent** | Eve-native child under `agent/subagents/<name>/`. Fresh context for decomposition. Collapsed under owning agent in fleet audit. |
| **Actor** | Concrete agent that initiated a tool call or handoff. Approvals and audit resolve to an actor. |
| **Topology** | The entire body of the Fleetfile (agents, edges, ownership, approval policies, runtime.supervisor, etc.). |
| **Topology version** | Human-managed semver in `metadata.version`. Warn-only on drift. |
| **Revision** | Git commit SHA. The only hard pin for deploy, hot-load refusal, and audit. |
| **Hot-load** | Reload of implementation trees (`agents/*/agent/`, `shared/`) without changing topology. |
| **Contract** | Free-text description of a handoff or job (required). Structured form optional. |

---

## 2. Settled Design Decisions (Canonical)

| Area | Decision |
|------|----------|
| **Primary job** | Explicit, git-versioned ownership & accountability |
| **Accountability** | Retrospective by default; optional per-edge `requires_ack` |
| **Topology vs implementation** | Entire Fleetfile = topology; only agent/shared trees are hot-loadable |
| **Handoff surface** | Generated `edge_<name>` tools in caller only |
| **Supervisor** | Optional; when declared (`runtime.supervisor: true`) it provides the full guarantee set |
| **Approval** | Single named actor (`approver` + optional timeout); `"human"` accepted but non-blocking in v1 |
| **Scale** | 50 agents = hard v1 ceiling |
| **Handoff authority** | Edges are the single source of truth; `may_delegate` / `called_by` removed |
| **requires_ack** | Per-edge, default false; means explicit parent accept/reject via generated tools |
| **Contracts** | Free-text required; structured optional |
| **Edge identity** | Every edge has a unique `name:`; used in tool names |
| **Versioning** | Both `metadata.version` (human) + git SHA (hard pin); version-bump is warn-only |
| **Cycles** | Hard error |
| **Edge direction** | Any agent → any agent, provided the graph is acyclic |
| **Generated tool location** | `agents/<name>/agent/tools/fleet/` |
| **Edge tool surface** | Input `{payload, context?}`, Output `{result?, status, error?}`; passthrough + embedded contract |
| **Ack/reject tools** | Optional reason; callable only after edge completes; reject fails the outcome |
| **on_failure** | `parent_handles` / `retry` → parent_handles / `escalate` → fail+notify / `fail` → edge+outcome failed |
| **Hot-load refusal** | Semantic diff of topology fields |
| **Path** | Strictly `agents/<name>` |
| **Lock** | No Fleet.lock; git SHA is the lock |
| **Root VERSION file** | Removed |
| **Shared tools** | Inherit caller’s approval policy |
| **Subagents in audit** | Collapsed under owning agent |
| **Mandatory audit fields** | fleet, topologyVersion, gitSHA, edgeName, from, to, actor, payloadHash, status, error?, requiresAck, acked (bool+actor+ts), timestamps, outcomeId (when available) |
| **CLI presentation** | Cobra owns commands/flags; Bubble Tea + Lip Gloss own TTY color/interaction only |
| **Eve purity** | No fork; generators emit stock-Eve-compatible TypeScript tools and trees |

---

## 3. Dependencies & Compatibility

Eve-Rails is intentionally thin. It generates and operates; it does not re-implement agent runtime, command parsing, or terminal rendering from scratch. The three load-bearing dependencies below are part of the system design.

### 3.1 Vercel Eve (agent framework)

**Role:** Filesystem contract and durable agent runtime.

- **Package:** npm `eve` (published name and CLI binary are both `eve`).
- **Minimum:** Track a concrete preview version at implementation freeze (e.g. `>= 0.40.0` as of 2026-08-20). Eve remains in preview; Eve-Rails v1 accepts that the underlying API may still move and will follow with compatible minor releases.
- **Contract Eve-Rails must honor:**
  - An agent is a directory under `agent/` with conventional slots (`instructions.md`, `agent.ts`, `tools/`, `skills/`, `subagents/`, etc.).
  - Tools are TypeScript modules using `defineTool` from `eve/tools` (typically with Zod `inputSchema`). Filename determines the runtime tool name.
  - Subagents live under `agent/subagents/<name>/` and are native Eve constructs.
  - Generated artifacts must be operable by the stock `eve` CLI and runtime with no Eve patches.
- **Node:** Eve requires Node.js 24+. Fleet projects inherit this prerequisite.
- **Policy:** Eve-Rails never calls private Eve APIs. Prefer composing `eve` / Vercel deploy and session flows over re-implementing Workflows, Sandbox, or Agent Runs.

### 3.2 Go + Cobra (CLI framework)

**Role:** Command tree, flags, argument validation, shell completions, and process exit codes.

- **Go:** 1.22+ (or the minimum required by pinned dependencies at implementation time).
- **Cobra:** Pin to a current stable line (v1.10.x as of late 2025 / 2026). Prefer `RunE` over `Run`, `SilenceUsage` on error paths, and command groups for help output.
- **Responsibilities:**
  - Define the full `eve-fleet` verb surface (`init`, `agent`, `edge`, `doctor`, `dev`, `build`, `deploy`, `reload`, `status`, `audit`, …).
  - Global and per-command flags (`--json`, `--yes`, `--revision`, `--agent`, …).
  - Non-zero exit on validation failure.
  - Shell completions (agent names, edge names, roles) where the Fleetfile can be parsed.
- **Non-responsibilities:** Colored output, interactive forms, and live status views belong to the Charm layer, not Cobra.

### 3.3 Charm Bubble Tea + Lip Gloss (TTY presentation)

**Role:** Optional human-facing color, layout, and interaction when stdout is a terminal.

- **Stack:** Bubble Tea (Elm-architecture TUI), Lip Gloss (styling). Huh and Bubbles may be used for forms and components.
- **Activation rules:**
  - Used only when stdout is a TTY **and** `--json` is not set **and** `NO_COLOR` is unset.
  - Headless, CI, and `--json` paths must produce correct plain text or JSON with zero dependency on a TUI being present.
- **Typical surfaces:**
  - `doctor` — colored error/warning tables
  - `status` — topology version, SHAs, drift, supervisor state
  - `audit` — formatted accountability chain
  - `dev` — optional multi-agent coordination view (or thin orchestration around `eve dev`)
  - Interactive `agent add` / `edge add` — Huh-style forms when appropriate
- **Policy:** Presentation never becomes a source of truth. Topology logic, validation, and audit records must work identically with or without the TUI.

### 3.4 Dependency summary

| Dependency | Owns | Must not own |
|------------|------|--------------|
| **Eve** | Agent filesystem contract, durable runtime, tool discovery, subagents | Fleet topology, CLI verbs, colored UX |
| **Cobra** | Commands, flags, completions, exit codes | Colors, interactive state, agent execution |
| **Bubble Tea / Lip Gloss** | TTY color, layout, interactive views | Validation, topology, headless/CI behavior |

---

## 4. Overall Architecture

```
eve-fleet (Go + Cobra entrypoint)
    │
    ├─ TTY? ──► Bubble Tea / Lip Gloss presentation
    │
    ▼
Fleet repository (git)
  ├── Fleetfile                 ← topology (never hot-loaded)
  ├── agents/<name>/agent/      ← pure Eve trees (hot-loadable)
  │     └── tools/fleet/        ← generated edge + ack/reject tools (TypeScript)
  └── shared/                   ← hot-loadable implementations
        │
        ▼
Eve runtime (unchanged, stock `eve`) + optional supervisor
  Durable sessions · subagents · sandboxes · Vercel primitives
```

Eve-Rails generates valid Eve projects and adds topology, generation, and operational guarantees. It does not replace Eve, Cobra, or the terminal.

---

## 5. Fleetfile Schema (Normative)

This schema matches every settled decision.

### 5.1 Top-level

```yaml
apiVersion: eve.fleet/v1          # required
kind: Fleet                       # required

metadata:                         # required
  name: string                    # required, DNS-label (lowercase, hyphens)
  version: string                 # required, semver of topology (human-managed)
  description: string             # optional
  owners: [string]                # optional
  labels: {string: string}        # optional

shared:                           # optional
  skills: [string]
  tools: [string]
  connections: [string]

agents:                           # required, map keyed by agent name
  <agentName>: AgentSpec

edges:                            # list of EdgeSpec
  - EdgeSpec

runtime:                          # optional
  isolation: "strong" | "shared-sandbox-pool"   # default "strong"
  supervisor: bool                # default false; when true → full guarantees
  hot_load:
    agents: bool                  # default true
    shared: bool                  # default true
  git:
    required: bool                # default true
    pin: "commit"
```

### 5.2 AgentSpec

```yaml
agents:
  <name>:
    path: "agents/<name>"                    # required, strictly this form
    role: "parent" | "delegate"              # required

    owns:                                    # required
      # parent only
      outcome: string
      sla: string
      completion: string

      # delegate only
      job: string
      contract: string                       # free-text, required for delegate

    approval_policy:                         # optional
      approver: string                       # agent name or "human"
      timeout: string                        # optional Go-duration
      tools:
        <toolName>:
          approver: string
          timeout: string

    model: string                            # optional
    description: string                      # optional
```

**Validation:**

- Parent → `outcome`, `sla`, `completion` required; job/contract forbidden.
- Delegate → `job` + `contract` required; outcome/SLA/completion forbidden.
- `path` must equal `agents/<name>`.

### 5.3 EdgeSpec

```yaml
edges:
  - name: string                             # required, unique across fleet
    from: string                             # required
    to: string                               # required
    contract: string                         # required, free-text
    timeout: string                          # optional
    on_failure: "parent_handles" | "retry" | "escalate" | "fail"
    requires_ack: bool                       # default false
    description: string                      # optional
```

**Validation:** unique `name`; `from`/`to` exist; graph is a DAG (cycles = hard error). Any direction is allowed if acyclic.

### 5.4 Example (Revenue Ops)

```yaml
apiVersion: eve.fleet/v1
kind: Fleet

metadata:
  name: revenue-ops
  version: "1.4.2"
  description: "Inbound lead qualification and routing"
  owners: ["@revops", "@platform"]

shared:
  skills: [shared/skills/revenue-definitions]
  tools: [shared/tools/crm_read]
  connections: [shared/connections/salesforce]

agents:
  lead-intake:
    path: agents/lead-intake
    role: parent
    owns:
      outcome: "Inbound lead reaches qualified-or-rejected terminal state"
      sla: "P95 end-to-end < 15 minutes"
      completion: |
        Lead has either:
          - score ≥ 70 AND routed to an SDR queue, or
          - explicit rejection with reason code
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
      contract: |
        Input:  { email?, domain?, company?, raw: object }
        Output: { unique: bool, match_id?: string, confidence: number }

  enrich:
    path: agents/enrich
    role: delegate
    owns:
      job: "Enrich with firmographic and contact data"
      contract: |
        Input:  { lead: object, match_id?: string }
        Output: { enriched: object, sources: string[] }

  score:
    path: agents/score
    role: delegate
    owns:
      job: "Score lead against revenue rules and emit reasons"
      contract: |
        Input:  { enriched: object }
        Output: { score: number, reasons: string[], qualified: bool }

  route:
    path: agents/route
    role: delegate
    owns:
      job: "Route qualified lead to SDR queue or emit rejection"
      contract: |
        Input:  { lead: object, score: number, qualified: bool }
        Output: { action: "routed"|"rejected", target?: string, reason?: string }
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
```

---

## 6. Directory Layout

```
<fleet-root>/                          # git repository
├── Fleetfile
├── agents/
│   └── <name>/
│       └── agent/                     # pure Eve agent tree
│           ├── agent.ts
│           ├── instructions.md
│           ├── tools/
│           │   ├── ...                # hand-written tools
│           │   └── fleet/             # generated (TypeScript)
│           │       ├── edge_<name>.ts
│           │       ├── ack_edge_<name>.ts    # parents only
│           │       └── reject_edge_<name>.ts
│           ├── skills/
│           ├── subagents/
│           └── ...
├── shared/
│   ├── skills/
│   ├── tools/
│   ├── connections/
│   └── lib/
├── evals/
└── .eve-fleet/                        # generated artifacts
```

No root `VERSION` file. Generated tools must be valid Eve `defineTool` modules.

---

## 7. Runtime Semantics

### 7.1 Accountability

- **Default:** Retrospective. The audit trail attributes the outcome to the owning parent even if the parent never inspected intermediate results.
- **Optional:** `requires_ack: true` on an edge requires the parent to call generated `ack_edge_<name>` or `reject_edge_<name>` before the outcome can close.
- Reject fails the parent outcome. Timeout without ack/reject fails the edge and the parent outcome.

### 7.2 Generated edge tool

- **Path:** `agents/<caller>/agent/tools/fleet/edge_<name>.ts`
- **Name:** `edge_<name>` (Eve discovers by filename)
- **Shape:** Input `{ payload: object, context?: object }` → Output `{ result?: object, status: "ok"|"error", error?: string }`
- Description embeds the free-text contract. Passthrough in v1; records mandatory audit fields.

### 7.3 Ack / reject tools (parents)

- `ack_edge_<name>` / `reject_edge_<name>` with optional `reason: string`
- Callable only after the corresponding edge tool has completed
- Reject → parent outcome fails

### 7.4 on_failure

| Value | Behaviour |
|-------|-----------|
| `parent_handles` | Failure delivered to parent; outcome not auto-failed |
| `retry` | Supervisor retries small N then `parent_handles`; without supervisor = `parent_handles` |
| `escalate` | Edge hard-fails + notify approver; outcome failed |
| `fail` | Edge failed and parent outcome failed |

### 7.5 Optional supervisor

Activated by `runtime.supervisor: true` (CLI may override for local `dev`).

When present it must enforce timeouts, perform retry logic, block outcome completion until all `requires_ack` edges are resolved, ensure mandatory audit fields, and cooperate on topology hot-load refusal.

Without a supervisor those behaviours degrade to best-effort / audit-only.

### 7.6 Hot-load

- Allowed: `agents/*/agent/`, `shared/`
- Forbidden: any topology field in the Fleetfile
- Check: semantic diff of topology fields vs Fleetfile at deployed git SHA
- Emits audit event with from/to SHAs

### 7.7 Approval

- Single named actor (agent name or `"human"`) + optional timeout
- Shared tools inherit the **caller’s** approval policy
- `"human"` is non-blocking in v1 (audit note only)

### 7.8 Subagents

Eve `subagents/` for private decomposition. Fleet audit collapses subagent activity under the owning agent.

---

## 8. Audit Requirements

Every fleet handoff record must contain:

| Field | Notes |
|-------|-------|
| `fleet` | |
| `topologyVersion` | |
| `gitSHA` | |
| `edgeName` | |
| `from` / `to` | |
| `actor` | caller |
| `payloadHash` | content hash, not full payload |
| `status` / `error` | |
| `requiresAck` | |
| `acked` | bool + actor + timestamp when true |
| `timestamps` | started, completed |
| `outcomeId` | when available |

`eve-fleet audit` reconstructs the accountability chain from these fields.

---

## 9. CLI Surface (`eve-fleet`)

Cobra owns the command tree and flags. Bubble Tea / Lip Gloss own TTY presentation only.

| Command | Purpose |
|---------|---------|
| `init <name>` | Scaffold fleet repo + Fleetfile + git |
| `agent add <name> --role=parent\|delegate` | Generate agent directory + Fleetfile entry |
| `edge add --name --from --to --contract ...` | Add named edge |
| `shared … add` | Register shared capability |
| `doctor` | Validate topology, acyclicity, paths, ownership; colored table on TTY |
| `dev` | Local multi-agent development |
| `build` | Validate + project to deployable artifacts |
| `link` / `deploy [--revision=SHA]` | Deploy; record git SHA |
| `reload --agent=… [--shared]` | Hot-load implementation only |
| `status` | Topology version, SHAs, drift (rich view on TTY) |
| `audit [--outcome=ID \| --run=ID]` | Accountability chain |
| `version` | CLI version |

**Flags of general interest:** `--json`, `--yes`, `--non-interactive`, `--revision`, `--agent`, `--shared`.

**Presentation rules:**

- TTY + no `--json` + no `NO_COLOR` → Lip Gloss / Bubble Tea allowed.
- Otherwise → plain text or JSON only; exit codes and data identical.

---

## 10. Non-Functional Requirements

- **NFR-1** `doctor` on ≤ 50 agents completes in < 5 s on a typical developer machine.
- **NFR-2** Generated agents remain valid for stock `eve` CLI and runtime at the pinned Eve version.
- **NFR-3** No secrets in Fleetfile. Hot-load must not escalate privileges or change approval policies.
- **NFR-4** Diagnostics are actionable (path + rule + suggestion).
- **NFR-5** Build/deploy from the same git revision is deterministic.
- **NFR-6** Headless and `--json` modes produce correct output with no TUI dependency.

---

## 11. Acceptance Criteria (v1)

1. `init` + `agent add` (parent + delegates) + `edge add` produces a fleet that `doctor` accepts.
2. Generated agent trees (including `tools/fleet/*.ts`) are operable by stock `eve` for the pinned Eve version.
3. `doctor` rejects cycles, invalid paths, role/ownership mismatches, duplicate edge names, and topology/hot-load violations.
4. Topology changes cannot be applied via `reload`; implementation changes can.
5. `status` and audit output include topology version + git SHA + mandatory audit fields (plain and `--json`).
6. A Revenue-Ops-style example fleet can be created, validated, and (locally) exercised end-to-end.
7. When `runtime.supervisor: true`, the full guarantee set is provided; otherwise degradation is documented and observable.
8. All commands provide `--help` and exit non-zero on validation failure.
9. Colored / interactive output appears only on TTY without `--json` / `NO_COLOR`; CI and headless runs succeed without Charm initialization.

---

## 12. Open Implementation Items

These are realization details, not open design questions:

- Exact TypeScript templates for `edge_*` / `ack_*` / `reject_*` tools (`defineTool` + Zod)
- Concrete doctor error codes and message catalogue
- Supervisor wire protocol / process model
- Status and audit formatting (Lip Gloss tables vs Bubble Tea views)
- How generated tools record audit fields into Eve / Vercel Agent Runs
- Content-hash algorithm for `payloadHash`
- Exact retry count and backoff for `on_failure: retry`
- Pinned versions of `eve`, Cobra, and Charm modules at the moment of implementation freeze

These may be resolved in ADRs or implementation without changing this SRS.

---

## 13. Revision History

| Version | Date       | Notes |
|---------|------------|-------|
| 0.1.0   | 2026-08-20 | Initial draft from early design discussion |
| 0.2.0   | 2026-08-20 | Design-complete. Settled decisions Rounds 1–5. Clean schema. |
| 0.3.0   | 2026-08-20 | Integrated Dependencies & Compatibility (Eve, Cobra, Bubble Tea/Lip Gloss) as first-class system design. Updated constraints, architecture, CLI, NFRs, and acceptance criteria for uniform alignment. |

---

**End of SRS**
