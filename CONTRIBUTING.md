# Contributing to Vo

Thank you for your interest in contributing to Vo. This document explains how to get started and how to submit changes.

---

## Code of conduct

- Be respectful and constructive.

---

## Getting started

1. **Fork and clone** the dev branch.
2. **Build and run** (from the repo root):

   ```bash
   go mod tidy
   go build -o vo .
   ./vo
   ```

3. **Optional:** Adjust settings in the config file. On first run, Vo creates `vo.conf` in the system config directory (see README); edit that file to customize.

---

## Development setup

- **Go 1.21+** is required.
- Format and vet before submitting:

  ```bash
  go fmt ./...
  go vet ./...
  ```

- The project uses **build tags** for OS-specific code:
  - `internal/terminal/terminal_windows.go`: `//go:build windows`
  - `internal/terminal/terminal_unix.go`: `//go:build !windows`
  - Keep shared logic in `internal/core/` and put only terminal I/O in `internal/terminal/`.

---

## Project layout

| Area | Description |
|------|-------------|
| `internal/core/` | Buffer, editor loop, modes, config, keybinds, i18n. No OS-specific code. |
| `internal/terminal/` | `Terminal` interface and implementations. Platform-specific I/O and key parsing. |
| `main.go` | CLI, load config, create buffer/terminal/editor, run. |

When adding features, prefer extending the core and the `Terminal` interface rather than adding one-off OS branches in the core.


---

## Submitting changes

1. **Create a branch** from the dev branch.
2. **Make your changes**: keep commits focused and messages clear.
3. **Run** `go fmt ./...` and `go vet ./...`.
4. **Open a Pull Request** against the dev branch. Describe what you changed and why.
5. **Address review feedback** if any.

By submitting a PR, you agree that your contributions may be used under the project’s license.

---

## Code style

- Follow standard Go style ([Effective Go](https://go.dev/doc/effective_go), `gofmt`).
- Prefer clear names and short functions. Add a brief comment for non-obvious logic.
- Keep `internal/core/` free of OS-specific code; keep terminal and key/escape handling in `internal/terminal/`.
- New user-facing strings should go through the i18n layer in `internal/core/i18n.go` and the config `language` option where applicable.

---

## Questions

If something is unclear, open an issue with the “question” label (if available) or describe your question in a new issue.

Thank you for contributing.
