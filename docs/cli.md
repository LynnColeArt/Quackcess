# Quackcess CLI

## Commands

- `init [--name <project-name>] [--db <seed.duckdb>] [--skip-vector-setup] <project-path>`
  - Creates a `.qdb` project and copies/creates the embedded DuckDB file.
  - By default, `init` now runs vector provider setup (`install`) automatically.
  - Use `--skip-vector-setup` to defer model readiness checks and downloads until first manual run.

- `open [--no-ui] [--ui=<true|false>] <project-path>`
  - Opens a project and runs the Shell UI by default.
  - Use `--no-ui` or `--ui=false` to run in headless mode.

- `mcp [--principal <principal>] [--permission-matrix <path>] <project-path>`
  - Starts the MCP server for agent access.
  - `--permission-matrix` can point at a JSON file that defines per-principal tool allowlists.

```json
{
  "defaultAllow": false,
  "principals": {
    "analytics": ["*"],
    "alice": ["system.ping", "query.execute", "schema.inspect"]
  }
}
```

- `info <project-path>`
  - Prints manifest metadata and vector-provider status.

- `install`
  - Ensures the configured vector backend has the requested model available.
  - For HTTP backends, this checks catalogs and may download the model automatically when possible.

## Init + Vector Defaults

- Default vector provider profile is CPU-first local mode (`qwen-cpu` + `qwen3-embedding-0.6b`).
- If HTTP model provider is selected and `QUACKCESS_VECTOR_MODEL` is unset, the default model is `qwen3.5-0.8b`.
- In all cases where the backend is available, `init` calls the install path by default.

## Quickstart

```bash
# 1) Create a project from an existing DuckDB file.
quackcess init --name "MyProject" --db ./data/sample.duckdb ./workspace/myproject.qdb

# 2) Open the visual shell directly.
quackcess open ./workspace/myproject.qdb

# 3) Check current configuration from the project.
quackcess info ./workspace/myproject.qdb
```

If you used `--skip-vector-setup`, run install manually after `init`:

```bash
quackcess init --skip-vector-setup --name "MyProject" --db ./data/sample.duckdb ./workspace/myproject.qdb
quackcess install
quackcess info ./workspace/myproject.qdb
```

If vector setup fails during `init`, rerun `quackcess install` after fixing provider env variables.
