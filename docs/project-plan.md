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
4. Implement progress/error/loading states.
5. Introduce keyboard shortcuts and context actions.

Tests first:
- `state_transition_contract_test`
- `command_bus_contract_test`
- `ui_model_projection_contract_test`

### Phase 5 — Canvas editor and visual workflows

Goal: visual drag-and-drop query/table canvases.

Tasks:
1. Build canvas document model (position, selection, connections, semantic tags).
2. Implement canvas save/load against catalog artifact tables.
3. Add drag/drop operations for tables/fields/edges.
4. Wire SQL preview updates from canvas changes.

Tests first:
- `canvas_model_contract_test`
- `canvas_layout_roundtrip_contract_test`
- `canvas_to_sql_contract_test`

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
2. Vector provider default is deferred; no hardcoded provider.
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
- Vector provider: no hard default in v1; require explicit local embedding provider config (e.g., Ollama/qwen3.5-0.8b).
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
