# Quackcess Project Plan

**Target architecture:** Desktop-first Go application with GTK3 UI and DuckDB backend, built with a test-driven, contract-first approach.

## 1) Vision

Quackcess is a lightweight database application platform inspired by Microsoft Access, optimized for:

- DuckDB-first data storage and analytics
- Visual, canvas-based SQL/data workflows
- AI- and agent-friendly command/control via MCP
- Optional chart/report artifacts stored in project files
- Vector-aware workflows for semantic search and agent memory use cases

We are intentionally omitting classic Access forms in v1. The app focuses on:

- schema inspection and editing
- query design and SQL preview
- canvas-level modeling and visualization
- agent-aware metadata and real-time event propagation

## 2) Non-negotiable constraints

1. **GTK version is GTK3**
   - Use `gotk3` (or compatible binding) to avoid GTK4 instability on this target environment.
2. **Go-first implementation**
   - Core logic and orchestration in Go packages
3. **DuckDB as primary data engine**
   - No reliance on DB server-side stored procedures.
   - Use DuckDB tables/macros + Go orchestration where procedural behavior is needed.
4. **Project-first workflow**
   - Persist project state using `.qdb` package format.
5. **Test-driven, contract-first delivery**
   - Define behavior through tests before implementation.
6. **Agent integration is first-class from early phases**
   - MCP endpoint contracts are part of phase 1-2 planning.

## 3) Repository Organization

Suggested top-level layout:

- `cmd/quackcess/` — app CLI and entrypoint
- `internal/`
  - `project/` — `.qdb` manifest, pack/unpack, migrations
  - `db/` — DuckDB bootstrap, migrations, execution
  - `catalog/` — metadata object model and persistence
  - `query/` — query graph and SQL generation
  - `vector/` — vector metadata, embedding workflows, similarity search helpers
  - `report/` — chart/report artifacts and render specs
  - `mcp/` — MCP server/tools and event transport
  - `events/` — domain event bus and subscriptions
  - `appstate/` — state container for UI-core decoupling
  - `config/` — runtime config/env handling
- `ui/gtk/` — GTK3 views, presenters, widgets, canvas, command bus
- `pkg/` — exported APIs if needed later
- `docs/` — documentation and planning artifacts
- `tests/` — test fixtures, golden outputs, integration helpers

## 4) Ground Truth: What we are building first

### 4.1 Core objects

- Project: `.qdb` archive containing:
  - embedded DuckDB database file
  - app manifest
  - artifact payloads (canvases, charts/reports, query objects, vector workflows)
- Metadata objects in DuckDB/catalog:
  - tables, columns, relationships, views
  - query and canvas definitions
  - reports/charts
  - event log
  - vector definitions/jobs/provenance
- Execution objects:
  - SQL query text
  - action/job definitions (ordered step runners)
  - execution result envelopes (rows, schema, timing, error detail)

### 4.2 Eventing model

All durable mutations should emit events for:

- UI live refresh
- MCP subscription notifications
- activity/trace visibility

Minimum event categories:

- `project.opened`
- `schema.changed`
- `query.executed`
- `vector.job.updated`
- `artifact.updated`
- `error.recorded`

## 5) Phased execution (foundational to least foundational)

Each phase includes:
- Contract tests first (red)
- Implementation to satisfy contracts
- Integration and polish

### Phase 1 — Foundation, plan, and project shell

Goal: establish stable boundaries and project life-cycle basics.

Tasks:
1. Create `docs/project-plan.md` and technical design ADR skeletons.
2. Define manifest schema and project lifecycle contracts:
   - required members
   - supported schema version
   - packing/unpacking invariants
3. Implement `project` package for:
   - initialize/open project
   - list project contents
   - manifest validation
4. Add CLI commands for:
   - `init`
   - `open`
   - `info`
5. Add CI scaffolding + test runner + lint baseline.

Tests first:
- `project_manifest_contract_test`
- `project_pack_unpack_contract_test`
- `project_version_migration_contract_test`

### Phase 2 — DuckDB core and metadata catalog

Goal: create reliable storage and object metadata contract.

Tasks:
1. Implement DuckDB bootstrap + migration harness.
2. Define catalog schemas for system state.
3. Add deterministic DB connection and error-normalization layers.
4. Implement metadata repositories for tables, columns, relationships, views, and canvases.
5. Add event emission on catalog mutations.

Tests first:
- `db_bootstrap_contract_test`
- `migrations_are_idempotent_contract_test`
- `catalog_table_crud_contract_test`
- `migration_versioning_contract_test`
- `error_shapes_contract_test`

### Phase 3 — Query model and SQL generation

Goal: provide the first true Access-like data design loop.

Tasks:
1. Define graph model for query nodes and relationships.
2. Implement SQL generator with deterministic ordering and escaping.
3. Add query history and explain capture.
4. Add result envelope and row serialization abstraction.
5. Add preview update pipeline (model -> SQL -> validation).

Tests first:
- `query_graph_contract_test`
- `sql_generation_contract_test`
- `query_execution_contract_test`
- `sql_preview_roundtrip_contract_test`

### Phase 4 — UI shell and core interactions (GTK3)

Goal: replace stub-only state with interactive app shell.

Tasks:
1. Implement main window and navigation regions.
2. Add Explorer/panel layout:
   - object tree
   - canvas viewport
   - SQL preview
   - results grid
3. Add command bus and state store bindings.
4. Add UI shell projection model for renderer-facing state (`internal/ui/shell`).
5. Implement progress/error/loading states.
6. Introduce keyboard shortcuts and context actions.
7. Add GTK bridge seam (`internal/ui/gtk`) for terminal submit + F12 shortcut dispatch.
8. Add terminal command service contracts:
   - execute SQL from terminal input
   - show query/error results
   - expose `\history` and in-app command hints
   - toggle event console with F12

Tests first:
- `state_transition_contract_test`
- `command_bus_contract_test`
- `ui_model_projection_contract_test`
- `shell_bridge_contract_test`
- `shell_window_contract_test`
- `terminal_contract_test`
- `event_console_contract_test`

Phase 4 status: complete for core shell interactions.

- Completed:
  - UI projection model, command bus wiring, terminal result formatting
  - explorer/main/workspace shell layout
  - live catalog browsing in shell explorer projection (tables/views/canvases)
  - canvas viewport scaffold and explorer interactions (table/view open actions)
  - terminal-level `\canvas <name>` execution path used by canvas explorer actions
  - F12 console toggle wiring
  - CLI-open headless fallback behavior
- Remaining:
  - full drag/select/canvas edit interactions in the canvas editor (now moving into Phase 5)

### Phase 5 — Canvas editor and visual workflows

Goal: visual drag-and-drop query/table canvases.

Phase 5 is now split into executable blocks we can complete in order:

#### 5.0 — Canvas editability foundation (domain first)

Tasks:
1. Expand the canvas artifact model beyond raw JSON by adding an explicit in-memory document model:
   - `CanvasDocument` with stable identity, schema version, title, description, tags
   - node coordinates, dimensions, and field selections
   - edge metadata and semantic labels (join fields, join direction, cardinality)
   - selection snapshot (active node/edge, multiselect hints)
2. Add deterministic normalization + validation contracts for the domain model:
   - alias uniqueness and collision handling
   - position and size normalization
   - edge endpoint/column existence validation
3. Add a small projection helper for canvas-to-SQL preview state (not just execution).

Tests first:
- `canvas_document_contract_test`
- `canvas_layout_roundtrip_contract_test` (expanded for node/edge metadata)
- `canvas_selection_contract_test`
- `canvas_projection_contract_test`

Status: contract coverage started and in-progress work completed for:
- `canvas_document_contract_test`
- `canvas_layout_roundtrip_contract_test`
- canvas normalization and edge-join validation in `internal/query/canvas_model.go`
- `canvas_selection_contract_test`
- `canvas_projection_contract_test`

#### 5.1 — Canvas persistence and artifact API

Tasks:
1. Add canvas repository methods needed for editing loops:
   - `GetByID`, `FindByName`, `Update`, `Upsert`, `Delete`
   - optional optimistic concurrency via version/timestamp checks
2. Add a dedicated artifact service facade for canvas writes:
   - normalize + validate before save
   - persist `spec_json` and metadata
   - expose `ListByKind`, `GetForExecution`, and `History` helpers
3. Add migration-safe `canvases` metadata columns for versioning and source references.

Tests first:
- `canvas_repository_contract_test`
- `canvas_artifact_contract_test`
- `canvas_service_contract_test`

Status:
- `canvas_repository_contract_test` added with `GetByID`, `ListByKind`, `Update`, and `Upsert` contracts.
- `internal/catalog/canvas.go` now implements those repository methods and stores `version/source_ref/updated_at`.
- `internal/db` migration path upgrades canvas metadata from schema `1.0.0` to `1.1.0`.
- `internal/canvasservice` provides artifact-facing behavior:
  - draft create
  - versioned rename/save/delete
  - normalized `\canvas save`
  - `GetForExecution`, `ListByKind`, `History`
- `canvas_artifact_contract_test`
- `canvas_service_contract_test`

Next slice to execute:
1. Add canvas edit lifecycle contracts for in-place mutation (drag/drop, field selection, and edge edits).
2. Add shell-facing state refresh after mutation commands.
3. Add draft live-preview contracts for SQL updates.

#### 5.2 — Workspace UI for canvas browsing/editing

Tasks:
1. Promote the canvas viewport from read-only scaffold to full workspace surface:
   - canvas list + active canvas selector from shell projection
   - save/load/revert state controls
   - status and validation banner (warnings/errors)
2. Add object palette in GTK:
   - table node creation
   - field pick list
   - quick edge/join creation action
3. Add non-drag interactions first (selection, delete, clear, rename, run, copy SQL preview, open related).

Tests first:
- `shell_canvas_panel_contract_test`
- `shell_canvas_toolbar_contract_test`
- `canvas_panel_shortcuts_contract_test`

#### 5.3 — Drag-and-drop and interaction engine

Tasks:
1. Add pointer-driven move/select model for nodes and fields.
2. Add edge drawing and attach workflow:
   - source/target selection
   - join-column picker
   - join-type chooser
3. Add canvas command model hooks:
   - `\canvas` execution by name already implemented in terminal
   - add `\canvas save|rename|delete` and `\canvas new` skeletons (contract-first)
4. Persist intermediate edits as draft/autosave snapshots.

Tests first:
- `canvas_drag_drop_contract_test`
- `canvas_edge_edit_contract_test`
- `canvas_command_contract_test`

Status:
- `canvas_drag_drop_contract_test` and `canvas_edge_edit_contract_test` are green.
- `canvas_command_contract_test` is now added and green, covering service-availability and malformed-command contracts for terminal canvas lifecycle commands.

#### 5.4 — SQL preview and execution feedback loop

Tasks:
1. Add live `sql_preview` projection from `\canvas` document mutations.
2. Execute preview safely with:
   - deterministic quoting and limit defaulting
   - error capture with source mapping to canvas nodes/edges where possible
3. Add result surface in shell viewport:
   - SQL generated preview pane
   - parameter list and estimate hints
   - rerun + cancel actions

Tests first:
- `canvas_sql_preview_contract_test`
- `canvas_execution_feedback_contract_test`
- `terminal_canvas_contract_test` (expanded for refresh behavior)

Current status: complete.
- Added contract coverage for active-canvas SQL previews, mutation refresh behavior, and malformed-spec safety.
- Added execution-feedback contracts for preview failures and terminal `\canvas` runs with deterministic error mapping.
- Verified shell model projection reflects live draft changes and run/readiness states.

### 5.5 — UI and user-flow polish

Goal: close remaining usability gaps from interactive editing.

Planned tasks:
1. Re-evaluate status/validation surfacing for save/revert/new/delete.
2. Tighten default startup ergonomics (including console visibility defaults and initial focus choices).
3. Final review pass on interaction keyboard shortcuts and feedback text.

Status:
- Canvas viewport interactions, node/edge editing actions, and preview updates are implemented.
- Shortcut wiring and command surface behavior are in place (including F12 console toggle).
- `\canvas new` status surfacing now updates `CanvasStatus` for both terminal command and action flows (`canvas created: <name>`), with contract coverage in appstate and shell model tests.

### Proposed Phase 5 completion gate

- All Phase 5 contracts pass with stable results for:
  - create/edit/save/load canvas
  - live SQL preview generation
  - drag/drop node/edge editing
  - execute/save/inspect roundtrip through terminal command path.

Completion signals before Phase 6:
1. No terminal-only route for core canvas workflows; canvas mutations happen in-shell with immediate SQL visibility.
2. Explorer canvas open is execution-capable (already completed) and no longer scaffold-only.
3. At least one “open project → create canvas → edit → run → persist → reopen” E2E scenario is contract-tested and green.

### Phase 6 — `.qdb` artifacts and migrations

Goal: package all report/query/canvas artifacts in a robust portable format.

Tasks:
1. Finalize artifact schema for:
	- canvas
	- charts
	- reports
	- macro/action steps
2. Add schema-versioned artifact migration.
3. Implement conflict resolution rules for duplicate IDs and stale manifests.
4. Add export/import utility with integrity checks.

Tests first:
- `artifact_schema_contract_test`
- `artifact_id_collision_contract_test`
- `artifact_pack_unpack_contract_test`
- `qdb_backward_compat_contract_test`

Current status:
- We started with `artifact_schema_contract_test` and the following Phase 6 contracts are now implemented:
  - `ArtifactKind` enum/validation
  - canonical manifest path construction
  - phase-1 artifact spec shape (`ArtifactSpecV1`) with validation
  - artifact path collision detection and canonical-path stability
  - kinded artifact pack/unpack roundtrip
  - manifest migration during `.qdb` open for backward compatibility

### Phase 7 — MCP control plane (agent-friendly)

Goal: safe remote control + live observability.

Tasks:
1. Implement MCP tool registry and transport wiring.
2. Add secure command surface:
   - query execution
   - schema inspection/mutation
   - artifact mutation
   - subscriptions
3. Add streaming/subscription channel for events.
4. Add permission/allowlist and audit log.

Tests first:
- `mcp_tool_contract_test`
- `mcp_authz_contract_test`
- `mcp_event_stream_contract_test`
- `mcp_error_surface_contract_test`

Current status:

- MCP phase-7 contracts are implemented and passing.
- In-package MCP core currently provides:
  - tool registration + deterministic listing
  - authorization checks and allowlist behavior
  - event streaming for tool call start/success/failure
  - consistent error codes for unknown tool, unauthorized, missing request, invalid args, and handler failures
- Transport hookup with the Go MCP SDK and stdio CLI entrypoint are now implemented.

### Phase 8 — Vector workflows

Goal: make vectors easy and transparent for AI workflows.

Tasks:
1. Define vector field and embedding metadata model.
2. Implement embedding job orchestration interface (provider-agnostic).
3. Add similarity query primitives and result scoring contract.
4. Add stale-index detection and rebuild controls.

Tests first:
- `vector_field_contract_test`
- `vector_dimensionality_contract_test`
- `vector_similarity_contract_test`
- `vector_reindex_contract_test`

Current status:
- Vector field metadata model and catalog persistence are now implemented and stable:
  - `internal/vector` has metadata model canonicalization, search primitives, and stale-index logic contracts.
  - `internal/catalog` has full vector-field CRUD (`Create`, `GetByID`, `List`, `Upsert`, `Delete`) with contract coverage.
  - DB bootstrap/migration now includes `quackcess_vector_fields`.
- Phase 8 is green through metadata/search primitives.
- Next slice: provider-agnostic embedding execution/service contracts and vector build/rebuild workflow.

- Delivered slice:
  - Added provider-agnostic embedding orchestration (`EmbeddingProviderRegistry`, `VectorBuildService`) with contract tests in `internal/vector/orchestration_contract_test.go`.
  - Added MCP exposure for vector field discovery (`vector.list`) and wired it into CLI MCP bootstrap (`cmd/quackcess/main.go`), keeping vector metadata first-class for agent workflows.

### Phase 9 — Charts/reports layer

Goal: analytics-first report and chart definitions.

Tasks:
1. Define chart/report DSL and schema.
2. Bind charts to SQL/query artifacts.
3. Add parameterized filters and grouping rules.
4. Implement deterministic export pipeline (CSV + image placeholders for render step).

Tests first:
- `chart_spec_contract_test`
- `report_composition_contract_test`
- `report_render_contract_test`
- `export_contract_test`

Current status:
- `internal/report` foundation is in place with schema, parse/canonicalize/marshal helpers.
- `chart_spec_contract_test` and `report_composition_contract_test` are implemented and passing.
- `report_render_contract_test` and `export_contract_test` are implemented and passing.
- Added: project-facing binding in `internal/project` now resolves chart/report artifacts from `.qdb`, loads their specs, and resolves report render plans.
- Added: project-facing `LoadReportExport` contract to generate deterministic export payloads (CSV + image placeholders) from report artifacts and per-chart row data.

### Phase 10 — Hardening and release readiness

Goal: production-grade reliability and maintainability.

Tasks:
1. Failure recovery and partial-write protections.
2. Security hardening for SQL and MCP entry points.
3. Performance baseline and regression checks.
4. Cross-platform packaging.

Tests first:
- `corruption_recovery_contract_test`
- `permission_matrix_contract_test`
- `performance_budget_contract_test`
- `packaging_workflow_contract_test` (via `internal/packaging` checks)
- release artifact workflow contract (via `.github/workflows/release.yml`)

Phase 10 status:
- Completed:
  - `corruption_recovery_contract_test`
  - `permission_matrix_contract_test`
  - `mcp` now accepts `--permission-matrix` and loads a JSON principal/tool allowlist.
  - denied MCP calls emit `mcp.call.denied` audit events.
  - `performance_budget_contract_test` (CLI cold-start, large query latency, shell responsiveness budgets).
  - cross-platform packaging checks (CI matrix + workflow contract in `internal/packaging`).
  - release artifact workflow (Linux/macOS tar.gz + checksums upload to GitHub release).

## 5b) Decomposition by dependency (execution order)

### Priority bands

1. **Foundation & contracts (must-complete before everything else)**
   - project manifest and `.qdb` format
   - DB bootstrap and migrations
   - event contract and logging envelope
   - MCP core tool contracts
2. **Core data workflows (enables all higher features)**
   - table/relationship metadata
   - query graph + SQL generation
   - result model and execution contract
3. **UI/interaction layer (enables direct usage)**
   - shell/state architecture
   - canvas model + SQL preview
   - table/query object management
4. **Agent loop + analytics**
   - MCP subscriptions/events
   - chart/report objects
   - vector workflows
5. **Stability and packaging**
   - recovery behavior
   - migration backfills
   - cross-platform build/distribution

### Phase 1 implementation decomposition (the first code slice)

- `docs/project-plan.md` complete (done).
- Add schema:
  - `internal/project/manifest.go`
  - `internal/project/pack.go`
  - `internal/project/manifest_test.go`
- Add minimal CLI:
  - `cmd/quackcess/main.go`
  - `cmd/quackcess/init.go`
  - `cmd/quackcess/open.go`
- Add test scaffolding:
  - `go.mod`, `.golangci.yml`, `.gitignore`
  - `docs/internal/testing-guidelines.md`
- Optional build and CI:
  - `.github/workflows/test.yml`

### Decomposition rule

For each phase:
1. implement one package completely before adding dependent packages
2. keep API boundaries small and pure
3. avoid adding UI surface area before data contracts are stable
4. never merge multiple slices in one PR without at least one contract test per public boundary

### Exit criteria per phase

- Phase complete only if:
  - all contract tests in that phase pass
  - package-specific docs are updated
  - any generated artifacts are migration-safe
  - one end-to-end smoke scenario is documented and green

## 6) Contract-first test strategy

For each component we follow:

1. **Define interface contract tests** (pure behavior)
2. **Add one implementation** to satisfy tests
3. **Refactor** while keeping contract coverage green

Testing levels:

- Unit contracts per package
- Integration for `.qdb` pack/unpack + DuckDB migrations
- Event-driven behavior tests for UI/MCP sync
- Smoke + golden tests for command execution paths

## 7) MVP scope boundaries (keep us focused)

MVP excludes (initially):

- Access-style form designer
- Full SQL editor features like advanced formatters, SQL autocomplete engine
- Report rendering engine with every chart type
- Multi-user concurrency server mode

These are intentionally deferred until after core loop stabilizes.

## 8) Immediate assumptions / open items (resolve before coding phase 1.2)

1. MCP transport is resolved for phase 1: **stdio + optional streamable HTTP**.
2. Vector provider default is enabled for local-first run: `qwen-cpu` + `qwen3-embedding-0.6b` with HTTP override support via `QUACKCESS_VECTOR_BACKEND=http` and a llama.cpp convenience alias via `QUACKCESS_VECTOR_BACKEND=llama`/`llamacpp`.
3. Authentication model resolved:
   - stdio: process-local transport, no external auth required.
   - streamable HTTP: localhost bind by default, optional token auth for non-local access.
4. OS targets resolved: **Linux and macOS**.
5. Chart/export formats confirmed for v1 data export (`CSV` + `JSON`); image export options pending.

## 8b) Current decisions and recommended artifact strategy

### MCP in Go

- MCP is formally supported for Go by the **official Tier-1 Model Context Protocol Go SDK** (`modelcontextprotocol/go-sdk`).
- Recommended transport for v1:
  - **stdio** for local-in-process agent launch/debug workflows.
  - **Streamable HTTP** as an optional second transport for remote live-agent access and streaming events.
- This gives us direct support for both local and remote control without building a transport wrapper first.

### Platform and provider constraints (now decided)

- OS targets: **Linux and macOS**.
- Vector provider: default local profile enabled (`qwen-cpu` + `qwen3-embedding-0.6b`) with env override support and optional local llama-compatible profile (`llamacpp`, `qwen-cpp`, `qwen3-embedding-0.6b`).
- Artifact IDs: **ULID** for deterministic, URL-safe, sortable identifiers.

### Artifact strategy (recommended)

Use a **dual-layer artifact model**:

1. **DuckDB registry tables for canonical metadata**
   - object identity, type, schema-version, dependencies, checksums.
2. **`qdb` sidecar files for artifact payloads**
   - `artifacts/<artifact-id>/manifest.json` and optional text/JSON payload files.

Core artifact kinds (v1):

- `canvas`: table/query graph layout and node metadata.
- `query`: query graph + SQL preview snapshot.
- `chart`: chart descriptors and rendering hints.
- `report`: ordered chart/sections + parameters.
- `procedure`: ordered action/workflow definitions.

Chart format strategy:

- **Mermaid as first-class v1 format** for flow/ER/sequence/report-like visuals.
- **Vega-Lite** as the second visual stack for expressive data charts.
- Keep `D2` and `PlantUML` as optional import/export adapters rather than defaults.

Open item after lock:

- Image export options in chart/report pipeline.

## 8c) Locked decisions (linked ADR)

- ADR: [Mermaid/Vega-Lite + MCP transport + ULID IDs](/home/lynn/projects/Quackcess2/docs/internal/adr-0002-visualization-and-mcp.md)
