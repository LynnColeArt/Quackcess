# Testing Guidelines (Phase 1-4)

## Principles
- Write the contract test first for each package.
- Keep contract tests focused on public package behavior.
- Prefer table-driven tests for multiple edge cases.
- Do not add implementation details in contract tests.

## Current contract set (Phase 1)

1. Project manifest parsing/validation.
2. .qdb package packing/unpacking shape and required members.
3. CLI command argument parsing for `init`, `open`, `info`.
4. `open` execution mode flags:
   - `--no-ui` skips the interactive terminal shell for headless flows.
   - `--ui` defaults true and can be set false; headless when false.
   - headless mode writes `open mode: headless` to stdout.
   - when interactive GTK UI is requested but unavailable, open prints `open mode: headless (ui unavailable)`.
   - UI mode writes `open mode: ui` to stdout.
5. Default behavior remains interactive: `open` (without `--no-ui`) runs through UI path.
   - default `open` path is validated by asserting the shell runner is invoked.

## Current contract set (Phase 2)

1. DuckDB bootstrap creates catalog schema and tracks schema version.
2. Bootstrap is idempotent and handles unsupported catalog versions explicitly.
3. Database errors are normalized into stable `*DBError` shapes.
4. Catalog table metadata CRUD contract (create/list/delete).
5. Catalog column metadata CRUD contract (create/list/delete).
6. Catalog relationship metadata CRUD contract (create/list/delete).
7. Catalog view metadata CRUD contract (create/list/delete).
8. Catalog canvas metadata CRUD contract (create/list/delete).

## Current contract set (Phase 3)

1. Query graph contracts (validation and normalization).
2. SQL generation contract for query graph inputs.
3. Query execution contract (result shape + row materialization).
4. Query history repository contract (logging + retrieval).
5. SQL preview roundtrip contract for generated query text.

## Current contract set (Phase 5)

1. Canvas model contracts:
 - JSON parse/normalize/validation for nodes, edges, join settings.
 - default alias behavior and join edge validation.
 - catalog-backed roundtrip stability for persisted canvas specs.
2. Canvas editability contracts:
 - canvas document normalization (nodes, edges, positions, selection state).
 - layout roundtrip including UI metadata (positions and aliases).
 - canvas validation diagnostics for invalid joins and stale node refs.
 - SQL preview contract from modified canvas documents.
3. Canvas-to-SQL contracts:
 - SQL generation from single-table layouts.
 - SQL generation from join-oriented layouts.
 - deterministic SQL with field aliases and default `SELECT *` fallback.
4. Canvas persistence contracts:
 - get-by-name / get-by-id repository behavior.
 - update/upsert semantics and artifact version bumps.
 - save/edit/load consistency from DB-backed storage.
 - `canvas_repository_contract_test` covers get-by-id, list-by-kind, update, upsert.
 - `canvas_artifact_contract_test` and `canvas_service_contract_test` cover versioned saves, normalized writes, and artifact-history behavior.
5. UI canvas workflow contracts:
 - canvas explorer-to-terminal execution behavior.
 - draft edit model and run/save actions.
 - drag/drop movement and edge connection contracts.

## Current contract set (Phase 4)

1. Terminal command service contracts:
   - query command execution and result shape
   - `\history` command and terminal output
   - `\canvas <name>` execution against persisted canvas specs
   - `\canvas new|rename|save|delete` mutation behavior
   - invalid command/error result contract
   - event console visibility + F12 toggle contract
2. Shell/app state contracts:
   - command bus dispatch for console toggling
   - terminal actions mutate shell projection
   - terminal failures propagate through command bus
   - terminal shortcut dispatch keeps projection synchronized
3. Shell projection contracts:
   - projection includes SQL text, output text, column names, and row previews
   - console state and status are reflected through projection
4. UI shell model contract:
   - command model projection for renderer bindings
   - GTK bridge contract for submit and shortcut dispatch
5. GTK shell-window contract:
 - in-memory shell window (non-`gtk3`) handles submit + F12 projection updates
 - run path reports build-availability in non-`gtk3` builds
 - native GTK3 shell window path (behind `gtk3` build tag) refreshes projection on submit + shortcut handling

5b. Live catalog explorer contracts:
 - `ShellProjection` includes table/view/canvas names
 - shell projection reflects catalog-backed lists when explorer provider is attached
 - terminal failures and console toggles do not break explorer rendering
 - open actions from explorer selections can trigger terminal execution paths

## Current contract set (Phase 6)

1. Artifact schema contracts:
   - artifact kinds are validated
   - artifact manifest paths are deterministic and namespaced by kind
   - phase-1 metadata validation for ids and schema versions
   - `artifact_contract_test.go`
2. Artifact identity and layout contracts:
   - canonical path collision detection in pack contracts
   - kinded artifact write/read consistency
   - `artifact_id_collision_contract_test.go`
   - `artifact_pack_unpack_contract_test.go`
3. Backward-compatibility contracts:
   - manifest migration during open for legacy artifacts
   - unsupported manifest versions rejected deterministically
   - `qdb_backward_compat_contract_test.go`

## Current contract set (Phase 7)

1. MCP tool contracts:
   - tool registration and list order guarantees
   - query execution contract
   - schema inspection contract
   - artifact set/get/delete/list contract
2. MCP authorization contracts:
   - default deny when allowlist is not configured
   - wildcard and tool-specific grants
   - cloned allowlist isolation
3. MCP event stream contracts:
   - lifecycle events for tool call start/success/failure
   - panic events map to failure results
   - event subscription and unsubscribe semantics
4. MCP error-surface contracts:
   - missing request/tool validation errors
   - invalid argument errors
   - handler errors and panic conversion to `handler_error`
5. MCP stdio transport contracts:
   - core tools are exposed through the Go MCP SDK transport
   - tool invocations preserve principal/request metadata
   - tool errors are returned as MCP tool errors (`IsError: true`)

## Current contract set (Phase 10)

1. `.qdb` corruption-recovery contracts:
   - create/upsert operations do not corrupt existing project archives on partial failure
   - required manifest + data references validated on open
2. MCP permission matrix contracts:
   - permission matrix JSON load and parse contract
   - wildcard principal and wildcard tool behavior
   - `--permission-matrix` CLI path forwarding and startup failure semantics
   - denied tool calls emit `mcp.call.denied` audit events
3. Performance budget contracts:
   - CLI cold-start (`open --no-ui`) budget
   - large query execution budget
   - UI shell responsiveness budget for large canvas interactions
4. Packaging workflow contracts:
   - CI packaging checks cover Linux and macOS matrix targets.
   - workflow runs a package build command so release tooling remains viable on supported OS targets.
5. Release packaging contracts:
   - tag-triggered release workflow builds Linux and macOS artifacts (`linux-amd64`, `darwin-amd64`, `darwin-arm64`).
   - release workflow creates versioned `.tar.gz` archives and `.sha256` sidecar checksum files.
   - generated assets are uploaded to a GitHub release.

## Conventions

- Test file naming: `<package>_contract_test.go` for top-level contracts.
- Use explicit fixture names in `internal/project/testdata`.
- Keep golden files small and deterministic.
