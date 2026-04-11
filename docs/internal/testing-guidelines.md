# Testing Guidelines (Phase 1)

## Principles
- Write the contract test first for each package.
- Keep contract tests focused on public package behavior.
- Prefer table-driven tests for multiple edge cases.
- Do not add implementation details in contract tests.

## Current contract set (Phase 1)

1. Project manifest parsing/validation.
2. .qdb package packing/unpacking shape and required members.
3. CLI command argument parsing for `init`, `open`, `info`.

## Conventions

- Test file naming: `<package>_contract_test.go` for top-level contracts.
- Use explicit fixture names in `internal/project/testdata`.
- Keep golden files small and deterministic.
