# Quackcess

Quackcess is a desktop-first, DuckDB-native data workspace inspired by Microsoft Access, built in Go with a GTK3 shell and an MCP control plane for AI agents.

It exists to make local analytics projects easier to build, inspect, and automate without needing a server-heavy stack or opaque toolchain.

## Why This Project Exists

Traditional Access-style workflows are still useful: fast schema iteration, visual query design, and portable project files. Quackcess keeps that workflow, but updates it for modern local analytics:

- DuckDB-first execution for fast local SQL analytics.
- `.qdb` project packaging for portable, versioned project state.
- Canvas-driven SQL workflows and artifacts.
- MCP tools and event streams so agents can safely inspect and operate on projects.
- Vector-aware workflows for semantic retrieval and memory-style use cases.

## What It Does Today

- Creates and opens `.qdb` projects with embedded DuckDB data.
- Provides a shell-style UI path (GTK3) with headless fallback.
- Exposes an MCP server with authz-aware tool surface.
- Supports canvas artifacts and canvas-to-SQL execution paths.
- Includes report/chart artifact foundations and export contracts.
- Includes vector field listing, index rebuild, and semantic search contracts.

## Current State Snapshot

This repo is actively developed with a contract-first test strategy.

- CI runs on Linux and macOS (`go test ./...`, `go build ./cmd/quackcess`).
- Release workflow packages Linux/macOS tarballs and checksum files on version tags.
- Major phases from foundational storage through MCP/vector/report/release hardening have contract coverage in-tree.
- Local test status in this workspace is currently green (`go test ./...`).

For detailed phase breakdown and decisions, see:

- `docs/project-plan.md`
- `docs/internal/testing-guidelines.md`
- `docs/internal/adr-0002-visualization-and-mcp.md`

## Quick Start

### Requirements

- Go `1.25.x`
- Linux or macOS
- GTK3 runtime if you want the interactive shell UI

### Build

```bash
go build -o quackcess ./cmd/quackcess
```

### Create and Open a Project

```bash
# create from an existing DuckDB file
./quackcess init --name "MyProject" --db ./data/sample.duckdb ./workspace/myproject.qdb

# open with UI (default behavior)
./quackcess open ./workspace/myproject.qdb

# force headless mode
./quackcess open --no-ui ./workspace/myproject.qdb

# show project and vector-provider info
./quackcess info ./workspace/myproject.qdb
```

If GTK3 is unavailable at runtime, `open` falls back to headless mode.

## MCP Mode (Agent Access)

Start MCP against a project:

```bash
./quackcess mcp ./workspace/myproject.qdb
```

You can provide a permission matrix JSON:

```json
{
  "defaultAllow": false,
  "principals": {
    "analytics": ["*"],
    "alice": ["system.ping", "query.execute", "schema.inspect"]
  }
}
```

Then run:

```bash
./quackcess mcp --permission-matrix ./permission-matrix.json ./workspace/myproject.qdb
```

## Vector Provider Notes

`init` runs vector setup by default. Use `--skip-vector-setup` to defer.

Common environment variables:

- `QUACKCESS_VECTOR_BACKEND`
- `QUACKCESS_VECTOR_ENDPOINT`
- `QUACKCESS_VECTOR_PROVIDER`
- `QUACKCESS_VECTOR_MODEL`
- `QUACKCESS_VECTOR_DIMENSION`
- `QUACKCESS_VECTOR_API_KEY`
- `QUACKCESS_VECTOR_TIMEOUT_SECONDS`
- `QUACKCESS_VECTOR_CPU_SEED`

Run setup manually:

```bash
./quackcess install
```

## Developer Workflow

```bash
go test ./...
go build ./cmd/quackcess
```

Key package areas:

- `cmd/quackcess`: CLI entrypoint and command wiring.
- `internal/project`: `.qdb` manifest and pack/unpack behavior.
- `internal/db`, `internal/catalog`, `internal/query`: core data model and SQL paths.
- `internal/ui/gtk`, `internal/ui/shell`, `internal/terminal`: interactive shell and projection/state plumbing.
- `internal/mcp`: MCP server, authz, tools, and event contracts.
- `internal/vector`: vector metadata, indexing, orchestration, and search contracts.
- `internal/report`: chart/report specification and export logic.

## Project Direction

Quackcess is intentionally not trying to be a full Access clone in v1. The focus is:

- local analytics speed
- visual SQL and canvas workflows
- agent-safe control surfaces
- portable project artifacts

If you want implementation-level context, start with `docs/project-plan.md` and follow the contract tests in each package.
