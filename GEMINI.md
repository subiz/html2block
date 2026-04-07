# GEMINI.md - html2block Mandates

This document serves as the foundational instruction set for all AI agents working on the `html2block` project. These rules take precedence over general defaults.

## Project Structure

- `html2block.go`: The primary Go implementation.
- `test-data/`: Shared HTML and JSON snapshots for integration testing.

## Core Implementation Mandates

1. **Parity:** Maintain functional parity between the Go and JavaScript versions. If the logic is updated in one, the other MUST be synchronized.
2. **Block Format:** The `Block` struct in `html2block.go` defines the schema for all conversions. Any new fields MUST be added to both versions.
4. **Collapsing Logic:** The `collapseBlock` function is critical for cleaning up the output structure. Be extremely careful when modifying it to avoid breaking nested structures like tables and lists.
5. **Tag Handling:** `tagTypeMaps` controls the initial mapping. Note that top-level `paragraph` types are converted to `div` at the end of `HTML2Block`.

## Testing & Validation

1. **Mandatory Testing:** Every change MUST be verified with `go test -v .`.
2. **Regression Testing:** Before committing changes, ensure that existing test cases in `html2block_go_test.go` still pass.
3. **Reproducibility:** If fixing a bug reported in the JS version, reproduce it in the Go version first with a new test case.

## Architectural Constraints

- Use `golang.org/x/net/html` for HTML parsing in Go.
- Avoid external dependencies for core conversion logic beyond the Go standard library and `x/net/html`.
- Maintain the "surgical" nature of `cleanBlock` to ensure the final JSON is as compact as possible.
