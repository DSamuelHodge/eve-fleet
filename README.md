# eve-fleet

**Eve gives you the agent. Eve-Rails gives you the org chart — with an audit trail.**

Greenfield Go CLI (`eve-fleet`) that makes **ownership, handoffs, and accountability** first-class and git-versioned for [Vercel Eve](https://github.com/vercel/eve) agent fleets.

This is **not** a port of `eve-rails-cli` or `eve-rails-cli-go`.

## Primary job

> Who owned this outcome, under which topology revision and git SHA, and which actors performed which steps?

## Spec

- Parent issue (`ready-for-agent`): https://github.com/DSamuelHodge/eve-fleet/issues/1
- Frozen design (SRS v0.3.0, Rounds 1–5 closed): [docs/SRS.md](docs/SRS.md)
- Realization spec: [docs/SPEC.md](docs/SPEC.md)

## Shape (v1)

| Piece | Decision |
| --- | --- |
| Topology | Entire **Fleetfile** (`apiVersion: eve.fleet/v1`) |
| Lock | Git SHA (no `Fleet.lock`) |
| Handoffs | Named **edges** (DAG, unique `name`) |
| Layout | Strictly `agents/<name>/agent/` (stock Eve) |
| Generated tools | Caller-only `tools/fleet/edge_<name>.ts` |
| Gate | `eve-fleet doctor` |
| Hot-load | Implementation trees only; topology diffs refused |
| Ceiling | 50 agents; cycles are a hard error |
| TUI | Charm on TTY only; `--json` / `NO_COLOR` / CI are headless |

Ticket #2 (`init` + `doctor` on an empty fleet) is implemented. Later tickets are not.

## Develop

```bash
go test ./internal/clitest
go build -o eve-fleet ./cmd/eve-fleet
./eve-fleet init revenue-ops
cd revenue-ops && ../eve-fleet doctor
```

CLI tests invoke the `eve-fleet` binary (the spec seam). Ticket #2 is `init` + `doctor` on an empty fleet.
