# Eve-Rails v1 — Fleetfile, `eve-fleet` CLI, and git-versioned accountability

**Status:** ready-for-agent  
**Source:** Eve-Rails SRS v0.3.0 (2026-08-20), design tree closed (Rounds 1–5)  
**Product:** Eve-Rails  
**CLI:** `eve-fleet`  
**This is a greenfield realization.** Do not port, wrap, or reuse any existing `eve-rails` / `eve-rails-cli` / `eve-rails-cli-go` code, manifests, catalog.yml layout, or command names.

---

## Problem Statement

An organization can stand up one Vercel Eve agent by hand. A department of agents — parents who own outcomes, delegates who own narrow jobs, and named handoffs between them — cannot. Ownership, who may hand work to whom, which actor approved a tool call, and which topology revision and git SHA that happened under, live in people's heads or in ad-hoc READMEs. When something goes wrong, there is no honest answer to:

> Who owned this outcome, under which topology revision and git SHA, and which actors performed which steps?

Fleet operators also cannot validate that the graph is acyclic, that every parent actually owns an outcome/SLA/completion, that a hot-load will not silently change topology, or that generated agent trees are still valid stock Eve. Ceremony that exists only to make spinning agents up faster is not the job. Ceremony required for an honest audit trail is.

## Solution

Ship **Eve-Rails**: a Rails-inspired convention layer and Go CLI (`eve-fleet`) that treats a **Fleetfile** as the entire topology (agents, edges, ownership, approval policies, runtime.supervisor) and git as the lock.

Operators declare a fleet in YAML, add parent and delegate agents and named edges, and the CLI generates **stock Eve** agent directories under `agents/<name>/agent/` — including generated `edge_<name>`, `ack_edge_<name>`, and `reject_edge_<name>` tools. `doctor` is the gate. `reload` hot-loads implementation trees only and refuses topology diffs. `status` and `audit` reconstruct accountability from mandatory fields pinned to topology version + git SHA. Charm (Bubble Tea / Lip Gloss) may color a TTY; headless, CI, `--json`, and `NO_COLOR` paths work with no TUI. Eve is not forked. Max 50 agents. Cycles are a hard error.

## User Stories

1. As a fleet operator, I want to run `eve-fleet init <name>` and get a git repository with a valid Fleetfile, so that a new fleet starts as topology plus empty conventional layout rather than a pile of agent folders.

2. As a fleet operator, I want `init` to require git (default `runtime.git.required: true`, pin commit), so that revision is a real SHA from the first commit.

3. As a fleet operator, I want `eve-fleet agent add <name> --role=parent` to write a Fleetfile parent (outcome, sla, completion required) and scaffold `agents/<name>/agent/` as a stock Eve tree, so that outcome ownership is explicit before any tools exist.

4. As a fleet operator, I want `eve-fleet agent add <name> --role=delegate` to write a Fleetfile delegate (job + free-text contract required) and scaffold the same Eve path convention, so that every specialist has a checkable job.

5. As a fleet operator, I want `doctor` to reject a parent that declares job/contract, and a delegate that declares outcome/SLA/completion, so that role and ownership cannot be mixed.

6. As a fleet operator, I want every agent's `path` to be exactly `agents/<name>`, and `doctor` to hard-fail any other form, so that layout is auditable without a lookup table.

7. As a fleet operator, I want `eve-fleet edge add --name --from --to --contract` to append a named EdgeSpec, so that allowed handoffs are data, not prose.

8. As a fleet operator, I want every edge to have a unique `name` across the fleet, and `doctor` to reject duplicates, so that generated tool names cannot collide.

9. As a fleet operator, I want `from` and `to` to name existing agents, and `doctor` to reject dangling endpoints, so that an edge cannot point at fiction.

10. As a fleet operator, I want edges to allow any agent → any agent provided the graph is a DAG, and `doctor` to hard-error on cycles, so that handoff authority stays acyclic.

11. As a parent-agent owner, I want each outgoing edge to generate `agents/<me>/agent/tools/fleet/edge_<name>.ts` only on the caller, so that the callee is not given a tool it did not request.

12. As a parent-agent owner, I want that edge tool to accept `{payload, context?}` and return `{result?, status, error?}`, with the free-text contract embedded in the tool description, so that Eve discovers a passthrough handoff whose meaning is in the contract.

13. As a parent-agent owner, I want `requires_ack: true` on an edge to also generate `ack_edge_<name>` and `reject_edge_<name>` on the parent, so that I can explicitly accept or reject a completed handoff.

14. As a parent-agent owner, I want ack/reject to take an optional reason, to be callable only after the edge tool has completed, and for reject to fail my outcome, so that acknowledgement is not a rubber stamp.

15. As a parent-agent owner, I want timeout without ack/reject on a `requires_ack` edge to fail the edge and my outcome, so that silence cannot close an SLA.

16. As a fleet operator, I want `on_failure: parent_handles` to deliver failure to the parent without auto-failing the outcome, so that I can recover.

17. As a fleet operator, I want `on_failure: retry` to mean supervisor retries a small N then `parent_handles`, and without a supervisor to mean `parent_handles`, so that retry is not silently invented in local dev.

18. As a fleet operator, I want `on_failure: escalate` to hard-fail the edge, notify the approver, and fail the outcome, so that dangerous failures do not stall.

19. As a fleet operator, I want `on_failure: fail` to fail the edge and the parent outcome, so that terminal failure is explicit.

20. As a fleet operator, I want `eve-fleet shared … add` to register shared skills, tools, or connections in the Fleetfile `shared:` block, so that common capabilities are named once.

21. As a tool author, I want shared tools to inherit the **caller's** approval policy, so that a shared CRM write is approved as the acting agent, not as a fleet-wide bucket.

22. As a security reviewer, I want approval policy to be a single named actor (`approver` = agent name or `"human"`) plus optional timeout, optionally overridden per tool, so that “who approved this” resolves to one actor.

23. As a security reviewer, I want `"human"` to be accepted in the schema but non-blocking in v1 (audit note only), so that we do not pretend a HITL UI exists.

24. As a fleet operator, I want `eve-fleet doctor` to validate topology, acyclicity, paths, unique edge names, role/ownership shape, agent-count ≤ 50, and hot-load rules, so that I have one gate before build/deploy.

25. As a CI system, I want `doctor` on ≤ 50 agents to finish in under 5 seconds on a typical developer machine, so that the gate is cheap.

26. As a CI system, I want every failing diagnostic to include path + rule + suggestion, so that a red build tells a human (or agent) what to type next.

27. As a CI system, I want `doctor` and every other command to exit non-zero on validation failure, so that pipelines fail closed.

28. As a CI system, I want `--json` and `NO_COLOR` (and non-TTY stdout) to produce plain text or JSON with identical data and exit codes and **zero Charm initialization**, so that headless runs do not load a TUI.

29. As an interactive operator, I want Lip Gloss / Bubble Tea color and tables on TTY when `--json` is unset and `NO_COLOR` is unset, so that `doctor`, `status`, and `audit` are readable in a terminal.

30. As a fleet operator, I want `eve-fleet dev` to run local multi-agent development, optionally enabling supervisor behaviour for local guarantees, so that I can exercise handoffs without deploying.

31. As a fleet operator, I want `eve-fleet build` to run the same validation as `doctor` and project deployable artifacts deterministically from the current git revision, so that two builds of the same SHA match.

32. As a fleet operator, I want `eve-fleet link` / `eve-fleet deploy [--revision=SHA]` to deploy and record the git SHA, so that production is pinned to a revision, not a floating branch.

33. As a fleet operator, I want `eve-fleet reload --agent=… [--shared]` to hot-load only `agents/*/agent/` and `shared/`, so that I can patch implementation without a topology change.

34. As a security reviewer, I want `reload` to semantically diff topology fields against the Fleetfile at the deployed git SHA and refuse if anything in topology changed, so that hot-load cannot add edges, change approvers, flip supervisor, or rewrite ownership.

35. As a security reviewer, I want a refused hot-load to emit an audit event with from/to SHAs, so that the attempt is itself accountable.

36. As a security reviewer, I want hot-load to be unable to escalate privileges or change approval policies, so that an implementation patch cannot widen blast radius.

37. As a fleet operator, I want `eve-fleet status` to show topology version, git SHAs, drift, and supervisor state (rich on TTY, JSON otherwise), so that I can see whether production matches the Fleetfile I think I deployed.

38. As an auditor, I want `eve-fleet audit [--outcome=ID | --run=ID]` to reconstruct the accountability chain, so that I can answer the primary job question.

39. As an auditor, I want every handoff record to contain fleet, topologyVersion, gitSHA, edgeName, from, to, actor, payloadHash, status, error?, requiresAck, acked (bool + actor + ts), timestamps (started, completed), and outcomeId when available, so that the chain is complete without storing full payloads.

40. As an auditor, I want `payloadHash` to be a content hash of the payload, not the payload, so that audit logs are not a PII dump.

41. As an auditor, I want default accountability to be retrospective: the parent owns the outcome even if they never inspected intermediates, so that “the delegate did it” is not a valid shrug.

42. As an auditor, I want Eve subagent activity collapsed under the owning agent, so that private decomposition does not fork the accountability chain.

43. As a parent-agent owner, I want to use Eve `agent/subagents/<name>/` for intra-agent decomposition with a fresh context, so that a parent can specialize without becoming another fleet node.

44. As a fleet operator, I want `runtime.supervisor: true` to mean the supervisor enforces timeouts, retry, blocks outcome completion until `requires_ack` edges resolve, writes mandatory audit fields, and cooperates on topology hot-load refusal, so that the full guarantee set is one flag.

45. As a fleet operator, I want the absence of a supervisor to degrade those behaviours to best-effort / audit-only, and for that degradation to be documented and observable in `status`, so that I am never surprised that a timeout was not enforced.

46. As a fleet operator, I want `runtime.isolation` to default to `strong`, with `shared-sandbox-pool` as the alternative, so that isolation is an explicit choice.

47. As a fleet operator, I want `runtime.hot_load.agents` and `runtime.hot_load.shared` to default true, so that implementation reloads are on unless I turn them off.

48. As a fleet operator, I want `metadata.version` to be human-managed semver of topology, with drift warn-only, so that people can talk about “topology 1.4.2” while git SHA remains the hard pin.

49. As a fleet operator, I want no `Fleet.lock` and no root `VERSION` file, so that git SHA is the only lock.

50. As a fleet operator, I want Fleetfile `apiVersion: eve.fleet/v1` and `kind: Fleet` required, so that the document is self-describing.

51. As a fleet operator, I want `metadata.name` to be a DNS-label (lowercase, hyphens), so that fleet names are filesystem- and URL-safe.

52. As a platform engineer, I want generated artifacts to be valid for stock `eve` (defineTool, conventional `agent/` slots, no private Eve APIs, no Eve fork), so that `eve` CLI and runtime operate the trees unchanged.

53. As a platform engineer, I want Node 24+ inherited as a fleet-project prerequisite because Eve requires it, so that we do not pretend an older Node works.

54. As a CLI author, I want Cobra to own the command tree, flags, completions, and exit codes, so that presentation cannot become a source of truth.

55. As a CLI author, I want Bubble Tea / Lip Gloss (and optionally Huh / Bubbles) to own TTY color, layout, and forms only, so that validation logic is identical with or without a TUI.

56. As a CLI user, I want `--help` on every command and shell completions for agent names, edge names, and roles when the Fleetfile parses, so that the verb surface is discoverable.

57. As a CLI user, I want global flags `--json`, `--yes`, `--non-interactive`, `--revision`, `--agent`, `--shared` to behave uniformly, so that CI scripts do not special-case verbs.

58. As a CI system, I want `--yes` / `--non-interactive` to suppress all prompts, so that `huh` forms never block a pipeline.

59. As a fleet operator, I want `eve-fleet version` to print the CLI version, so that support can pin the tool.

60. As a fleet operator, I want a Revenue-Ops-style example (parent `lead-intake` + delegates `dedupe`, `enrich`, `score`, `route` + named edges including one `requires_ack`) to init, doctor-clean, and exercise locally, so that the SRS example is a living acceptance fixture.

61. As a fleet operator, I want `doctor` to reject a fleet with more than 50 agents, so that the v1 cognitive and performance ceiling is enforced.

62. As a fleet operator, I want contracts to be required free-text, with structured form optional, so that a handoff always has a human-readable meaning.

63. As a developer of generated tools, I want generated TypeScript to live under `tools/fleet/` and remain valid `defineTool` modules with Zod input schemas, so that Eve discovers them by filename.

64. As a security reviewer, I want no secrets in the Fleetfile, so that topology can live in git.

65. As a fleet operator, I want `build`/`deploy` from the same git revision to be deterministic, so that what `doctor` accepted is what shipped.

66. As an interactive operator, I want optional Huh-style forms on `agent add` / `edge add` only when stdout is a TTY and `--json` is off, so that interactive and scripted paths share validation.

67. As a platform engineer, I want Eve-Rails to compose `eve` / Vercel deploy and session flows rather than re-implement Workflows, Sandbox, or Agent Runs, so that we stay a thin generator and operator.

68. As an auditor, I want `status` and `audit` JSON to include topology version + git SHA + the mandatory audit fields, so that machines can ingest the same truth as humans.

69. As a parent-agent owner, I want my `owns.outcome`, `owns.sla`, and `owns.completion` to appear in generated `instructions.md`, so that the running agent is told the SLA it is accountable for.

70. As a delegate-agent owner, I want my `owns.job` and `owns.contract` to appear in generated `instructions.md`, so that the specialist cannot invent a broader job.

71. As a fleet operator, I want editing the Fleetfile by hand and re-running `doctor` / `build` to be a supported path, so that YAML is the source of truth, not only the CLI verbs.

72. As a fleet operator, I want topology changes to require a new git commit and a deploy, never a `reload`, so that the audit question always has a SHA.

## Implementation Decisions

- **Greenfield.** Realize SRS v0.3.0. Do not reuse existing `eve-rails-cli` / `eve-rails-cli-go` packages, YAML layouts (`manifests/agents.yml`, `catalog.yml`), binary name (`eve-rails`), or doctor-check vocabulary. Those products are a different model.

- **Primary job.** Explicit, git-versioned ownership and accountability. Reducing spin-up friction is a constraint (keep cognitive load low), not the job.

- **Binary and stack.** `eve-fleet`. Go 1.22+ (or dependency minimum at freeze). Cobra (stable v1.10.x line; `RunE`, `SilenceUsage` on errors, command groups). Charm Bubble Tea + Lip Gloss for TTY only; Huh/Bubbles allowed for forms. npm `eve` as the agent framework (pin a concrete preview, e.g. `>= 0.40.0` at implementation freeze). Node 24+ on generated projects.

- **Three-way ownership.** Eve owns the agent filesystem contract and runtime. Cobra owns commands, flags, completions, exit codes. Charm owns TTY presentation. Presentation is never a source of truth.

- **Topology is the Fleetfile.** Entire document: apiVersion, kind, metadata, shared, agents, edges, runtime. Only `agents/*/agent/` and `shared/` are hot-loadable.

- **Fleetfile schema** is normative as in SRS §5 (AgentSpec path/role/owns/approval_policy; EdgeSpec name/from/to/contract/timeout/on_failure/requires_ack; runtime isolation/supervisor/hot_load/git). Defaults: isolation `strong`, supervisor `false`, hot_load agents/shared `true`, git required `true`, pin `commit`, `requires_ack` `false`.

- **Handoff authority.** Edges are the single source of truth. No `may_delegate` / `called_by`. Edge identity is unique `name`, used in tool names.

- **Generated tool location and shape.** Caller only, under the Eve tools slot `tools/fleet/`. `edge_<name>`: Input `{payload, context?}`, Output `{result?, status: "ok"|"error", error?}`. Passthrough in v1; description embeds contract; records mandatory audit fields. Parents with `requires_ack` also get `ack_edge_<name>` and `reject_edge_<name>` (optional reason).

- **Accountability.** Retrospective by default. Optional per-edge `requires_ack`. Reject fails the parent outcome. Ack/reject only after the edge completes.

- **on_failure.** `parent_handles` | `retry` | `escalate` | `fail` with SRS §7.4 semantics. Retry without supervisor degrades to `parent_handles`.

- **Supervisor.** Optional. `runtime.supervisor: true` (CLI may override for local `dev`) enables the full guarantee set. Absence is best-effort / audit-only and must be visible.

- **Approval.** Single named actor + optional timeout; per-tool overrides; `"human"` non-blocking in v1. Shared tools inherit the caller's policy.

- **Versioning.** `metadata.version` (human semver, warn-only drift) + git SHA (hard pin for deploy, hot-load refusal, audit). No Fleet.lock. No root VERSION file.

- **Scale and graph.** 50 agents hard ceiling. Cycles hard error. Any edge direction if DAG.

- **Paths.** Strictly `agents/<name>`. Generated tools: `agents/<name>/agent/tools/fleet/`. Subagents: Eve-native `agent/subagents/<name>/`. Shared implementations under `shared/`. Generated operational artifacts under `.eve-fleet/`. `evals/` exists in layout.

- **CLI verbs.** `init`, `agent add`, `edge add`, `shared … add`, `doctor`, `dev`, `build`, `link` / `deploy`, `reload`, `status`, `audit`, `version`. Flags: `--json`, `--yes`, `--non-interactive`, `--revision`, `--agent`, `--shared`.

- **Hot-load refusal.** Semantic diff of topology fields vs Fleetfile at deployed git SHA. Implementation-only reload. Audit event on refuse.

- **Audit fields.** fleet, topologyVersion, gitSHA, edgeName, from, to, actor, payloadHash, status, error?, requiresAck, acked (bool+actor+ts), timestamps, outcomeId when available. Subagents collapsed under owning agent.

- **Eve purity.** Generators emit stock-Eve-compatible TypeScript (`defineTool`, Zod). No private Eve APIs. Prefer composing `eve` / Vercel deploy and session flows.

- **Headless.** TTY + no `--json` + no `NO_COLOR` → Charm allowed. Otherwise plain text or JSON; exit codes and data identical; no TUI init.

- **Open realization details** (not design questions; resolve in ADRs without changing this spec): exact TypeScript templates for edge/ack/reject tools; doctor error-code catalogue; supervisor wire protocol / process model; status/audit formatting; how generated tools record audit fields into Eve / Vercel Agent Runs; payloadHash algorithm; exact retry N and backoff; pinned eve/Cobra/Charm versions at freeze.

## Testing Decisions

**One seam: the `eve-fleet` process.** Tests invoke the installed/binary CLI the way an operator or CI would (args, flags, env, cwd, exit code, stdout/stderr) against a temporary git repository that is a fleet. Assert on:

- Exit codes
- JSON (when `--json`) or plain diagnostics (path + rule + suggestion)
- Fleetfile contents after verbs
- Generated Eve trees (existence, `defineTool` shape, contract text, tool names, no extra topology files)
- `doctor` accept/reject for each validation rule
- `reload` allowing implementation diffs and refusing topology diffs
- Absence of Charm/TUI behaviour when stdout is not a TTY, when `--json` is set, or when `NO_COLOR` is set

Do not test internal packages, helpers, or Charm view models as the acceptance seam. If a unit test exists, it is an implementation convenience, not the spec's gate.

**Good tests** assert external behaviour only: “this command on this fleet repo produces this exit code, this JSON field, this file, this refusal.” They do not assert function names, template filenames, or Cobra wiring.

**Prior art in this workspace:** none. This is greenfield. Bootstrap with a CLI-level test helper that runs `eve-fleet` in a temp dir with a stubbed or recorded `eve` where runtime execution is required, and with pure filesystem/JSON assertions for doctor/init/generate paths.

**Minimum fixtures:**

- Happy path: `init` + parent + delegates + edges → `doctor` green (SRS acceptance #1)
- Revenue-Ops example fleet as a fixture (SRS acceptance #6)
- Cycle, bad path, role/ownership mismatch, duplicate edge name, 51st agent, topology-changing `reload` — each red with actionable diagnostics
- `--json` / non-TTY / `NO_COLOR` produce no TUI and the same data
- Generated `tools/fleet/*.ts` parse as Eve `defineTool` modules
- Supervisor on vs off is observable in `status`

**NFR checks** that belong at this seam: `doctor` on a 50-agent fixture finishes under 5s; two `build`s of the same SHA are byte-stable for generated topology-derived artifacts.

## Out of Scope

Per SRS §1.3, v1 does **not** include:

- Replacing or forking the Eve runtime / compiler
- A multi-tenant control plane or SaaS dashboard
- Automatic mass generation of agents from natural language
- A custom workflow engine beyond Eve Workflows
- Non-git version-control backends
- Multi-fleet / hierarchical fleets
- Rich approval policies (any-of, all-of, ordered chains)
- Blocking human-in-the-loop UI (`"human"` is schema-valid and non-blocking)
- A separate lockfile (`Fleet.lock`)
- TUI-only operation
- Anything from prior `eve-rails-cli` / `eve-rails-cli-go` (catalog.yml, `eve-rails` binary, `apply`/`hotload`/`wizard` as those tools exist today)

Open implementation items in SRS §12 stay out of *design*; they are allowed as ADRs during realization.

## Further Notes

- **Glossary (use these words):** Fleet, Fleetfile, parent agent, delegate agent, edge, subagent, actor, topology, topology version, revision (git SHA), hot-load, contract, supervisor, `requires_ack`, outcome, SLA, completion, job.

- **Hard constraints:** ≤ 50 agents; cycles = hard error; topology changes forbidden on hot-load; path strictly `agents/<name>`; edges are SoT for handoffs; generated artifacts valid for stock Eve; CLI fully functional with zero TUI.

- **Example topology** in SRS §5.4 (Revenue Ops: `lead-intake` parent, four delegates, four named edges, `route_lead` requires_ack, supervisor true) is the canonical acceptance fleet.

- **Tracker:** this spec is the parent. Follow-on work should be tracer-bullet tickets against *this* document, not against the older CLI issues.

- **Seam confirmation:** the single test seam is the `eve-fleet` CLI process + the fleet git repo it mutates. If that is the wrong altitude, say so before implementation starts.
