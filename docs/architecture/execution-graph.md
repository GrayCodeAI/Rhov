# Hawk Execution Graph

## Status

The first read-only execution-graph projection is implemented. It does not
replace Hawk's scheduler or move runtime ownership into a graph database.

```text
runtime owners
  session persistence
  structured task store
  background task registry
  permission engine
  verification bridges
  graph observation journal
  Embedded token-pipeline runtime projection
  Eyrie operations projection
        |
        v
internal/executiongraph
        |
        v
eagle/graph
        |
        +--> hawk graph export
        |
        +--> authenticated daemon session graph API
        |
        +--> privacy-normalized, explicit Hawk Cloud sync
```

## Ownership

| Data | Source of truth | Graph behavior |
|---|---|---|
| Hawk sessions and messages | `internal/session` | Read-only projection |
| Structured tasks and dependencies | `internal/tool` task store | Read-only projection |
| Agent, shell, and monitor tasks | `internal/taskruntime` | Read-only projection |
| Permission verdicts | permission subsystem | Automatically summarized after tool permission and approval gates |
| Verification reports | verification engines/bridges | Built-in plan verification is automatically summarized; other engines can supply typed observations |
| Context compression operations | `internal/token` | Compression statistics are automatically captured during persisted-session context compaction |
| Response redaction quality | `internal/token` | Aggregate secret matches are automatically captured after persisted-session response redaction |
| Token usage and budget decisions | `internal/token` | Deduplicated provider usage updates the tracker; usage and next-turn budget decisions are automatically captured |
| Model routing and generation usage | Eyrie | Route and normalized usage operations are automatically captured after each persisted-session model turn |
| Observation history | `internal/graphjournal` | Append-only, privacy-safe runtime evidence |

The graph package never mutates these owners and never schedules work.

## Validated scheduling view

Hawk's structured task store now validates its blocking dependency graph before
answering `TaskList {"action":"ready"}`. The validator:

- caps the view at 1,000 tasks and 10,000 unique blocking edges;
- rejects missing blockers, self-dependencies, invalid task states, and cycles;
- ignores `related` and `parent-child` relationships for scheduling;
- returns stable, sorted topological waves plus the currently runnable pending
  tasks;
- fails closed when legacy callers request ready work from an invalid graph.

This is graph-driven readiness selection with an opt-in execution bridge. The
default agent/tool loop remains authoritative, but `hawk mission --from-tasks`
can now consume the validated waves, execute each wave with bounded parallel
workers, and persist a deterministic join record before any later wave starts.
If a wave fails, later waves remain pending rather than being speculatively
dispatched.
The same execution bridge is available to library callers through
`Mission.RunStaged(..., WithExecutionWaves(waves))`, so staged PRD,
verification, and fix pipelines can retain graph ordering without depending on
the CLI or TaskStore.

Graph-driven execution keeps the existing operational controls:

- the mission worker cap bounds fan-out;
- worktrees still provide per-feature isolation;
- mission persistence records each wave join durably;
- a portable `mission-graph.json` artifact is rewritten alongside `mission.json`
  so graph consumers can read mission state without reading mutable runtime
  structs;
- later waves are cancellation- and failure-sensitive;
- downstream tasks are never unlocked by a failed blocker.

## Portable topology

Nodes:

- `hawk/session/<id>` — persisted Hawk session;
- `hawk/task-request/<session>/<n>` — user task request, content represented by
  SHA-256 only;
- `hawk/task/<id>` — structured task;
- `hawk/runtime-task/<id>` — background agent, shell, or monitor task;
- `hawk/tool-call/<session>/<tool-use-id>` — tool invocation metadata;
- `hawk/policy/<id>` — permission verdict;
- `hawk/verification/<id>` — neutral verification result;
- `eyrie/route/<digest>` and `eyrie/generation/<digest>` — model route and
  normalized generation operations.

Edges:

- `contains` — session/task hierarchy and task-to-tool containment;
- `depends_on` — blocking task dependency;
- `references` — related tasks and cross-session linkage;
- `governed_by` — execution subject to policy verdict;
- `validated_by` — execution subject to verification result.

All nodes, edges, and events pass the shared contract validators. Edges whose
source or target node is absent are rejected.

## Privacy and bounded payloads

The export deliberately excludes:

- prompt and response bodies;
- tool arguments and tool result content;
- background task prompts and output;
- policy reason text;
- verification targets, messages, fixes, and evidence;
- absolute working-directory paths.

Where correlation is useful, sensitive values are represented by SHA-256
digests. Every node declares `data_classification=metadata_only`.

Hawk writes runtime observations to a per-session append-only JSONL journal.
The journal is stored with mode `0600` under Hawk's state directory and contains
only verdict metadata, aggregate verification counts, and SHA-256 digests. A
journal failure is logged and never changes a permission, approval, or tool
result.

Mission-mode graph execution persists a separate `mission-graph.json` artifact
beside `mission.json` in the mission directory. The artifact uses the same
portable `hawk.graph/v1` envelope, but it is mission-scoped rather than
session-scoped: mission node, feature nodes, and wave-join operations nodes are
rewritten after each mission run or wave join using only metadata and SHA-256
digests.

## CLI

```bash
hawk graph export [session-id]
  --repository <scope>
```

The default repository scope is derived from the saved session's working
directory basename.

## Daemon API and SDKs

The daemon exposes the same projection through the authenticated, read-only
endpoint:

```text
GET /v1/sessions/{id}/graph
  ?repository=<scope>
```

The endpoint uses the daemon's normal Bearer or `X-API-Key` authentication,
validates session and repository inputs before projection, and
returns the typed `hawk.graph/v1` envelope. The handler owns transport concerns
only; the Hawk composition root injects the existing graph builder, so CLI,
Cloud sync, and HTTP projections share one construction path.

The Go, Python sync/async, and TypeScript SDK clients expose this endpoint and
validate the returned graph topology before returning it to callers.

After connecting a project with `hawk cloud login` or `hawk cloud connect`, a
completed session snapshot can be uploaded explicitly:

```bash
hawk cloud graph sync [session-id]
  --repository <scope>
```

The cloud adapter hashes values behind sensitive attribute names, changes those
keys to `*_sha256`, enforces the 250-node/500-edge/500-event/900-total-fact and
1 MiB limits, and derives the sync ID from the prepared graph. Repeating the
same completed snapshot is therefore idempotent. Upload errors are reported to
the explicit command, but cloud availability never affects local execution.
Hawk does not upload prompt bodies, response bodies, tool arguments, tool
results, or other large artifacts.

Mission-scoped graph artifacts can use the same explicit path without being
pretended to be a Hawk session:

```bash
hawk graph export --mission-dir <mission-directory>
hawk cloud graph sync --mission-dir <mission-directory>
```

The CLI reads only `mission-graph.json`, validates its `hawk.graph/v1` schema
and all edge/event references, then applies the same privacy normalization and
Cloud limits. Mission graphs intentionally omit `sessionId`; they are durable
mission facts, not Cloud session telemetry.

## Runtime capture status

The central tool-execution seam automatically records:

- the final permission-engine outcome for every persisted-session tool call;
- enabled human-approval gate outcomes;
- aggregate results from `VerifyPlanExecution`;
- token-pipeline compression performed by persisted-session context compaction;
- secret matches removed from persisted-session responses;
- token usage summaries and budget decisions emitted from the central,
  deduplicated provider-accounting seam;
- Eyrie model route and usage reported for persisted-session turns.

Mission execution now also persists a portable graph artifact for local mission
runs. Plain `hawk mission` runs emit mission and feature execution nodes; the
graph-driven `hawk mission --from-tasks` path additionally emits operations
nodes for each deterministic wave join, including bounded completion, failure,
and blocked-downstream counts.

The embedded token-pipeline tracker contributes hourly, daily, session, and cost
limits to the pre-turn guard. Existing Hawk cost accounting and limits remain
authoritative and are updated first; the tracker observes the same deduplicated
request usage and provides the additional token-window decision.
