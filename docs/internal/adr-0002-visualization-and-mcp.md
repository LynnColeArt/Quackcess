# ADR: Visualization and Transport Decisions for Quackcess v1

## Status
Accepted

## Context
Quackcess needs two things that affect long-term architecture:

1. MCP transport and Go integration path
2. chart/diagram format selection
3. artifact identity strategy
4. platform baseline

## Decision

### 1) MCP and transport
- Use `modelcontextprotocol/go-sdk` for MCP server/client implementation.
- Use MCP `stdio` transport as primary in v1.
- Add optional `Streamable HTTP` transport in a later milestone for remote/interactive workflows.
- Security model:
  - `stdio`: process-local transport; no external auth required.
  - `Streamable HTTP` (when enabled): bind to localhost by default and require a shared bearer token or API key for non-local clients.

### 2) Visualization formats
- Use **Mermaid** as the first-class diagram format for visual canvas outputs and lightweight charts.
- Use **Vega-Lite** for analytical charting (bar/line/scatter, etc.).
- Do not add additional chart runtimes in v1 beyond Mermaid and Vega-Lite.

### 3) Vector provider model
- Default to a CPU-first local vector profile for v1 (`qwen-cpu` with `qwen3-embedding-0.6b`) when env vars are not provided.
- Keep env overrides (`QUACKCESS_VECTOR_BACKEND`, `QUACKCESS_VECTOR_ENDPOINT`, `QUACKCESS_VECTOR_PROVIDER`, `QUACKCESS_VECTOR_MODEL`, `QUACKCESS_VECTOR_CPU_SEED`) supported for easy provider swapping.
- CPU provider defaults are immediate and deterministic; backend `QUACKCESS_VECTOR_BACKEND=llama`/`llamacpp` maps to a local HTTP-compatible default profile (`qwen-cpp` + `qwen3-embedding-0.6b`) and defaults endpoint to `127.0.0.1:8080/v1/embeddings`.
- HTTP providers can be explicitly enabled with `QUACKCESS_VECTOR_BACKEND=http` for remote/local OpenAI-compatible endpoints (`qwen-local` + `qwen3.5-0.8b` default).

### 4) Platform and identity
- Target OS: **Linux + macOS**.
- Artifact IDs will use **ULID** for sortable, URL-safe identifiers.

## Consequences

- `project` artifact registry can remain transport-agnostic while storing compact metadata per artifact.
- Mermaid/JSON payloads are easy to diff and version in `.qdb` payload files.
- No immediate maintenance burden from multiple alternate chart ecosystems.
- ULID enables deterministic sorting of creation order for UI display and history feeds.
