# Vo

A minimal, terminal-based text editor in Go with a modal editing experience. Vo is **inspired by** [Vim](https://www.vim.org/) and [Neovim](https://neovim.io/)—it is not meant to replace or compete with them, but to offer a small, hackable codebase for learning and experimentation while keeping a familiar feel.

---

## Inspiration

Vo draws ideas from the modal editing model and keybindings of **Vim** and **Neovim**. We are grateful to those projects and their communities. Vo is a separate, minimal implementation in Go, intended as a learning project and a lightweight editor—not as an alternative to the full-featured editors we were inspired by.

---

## Features

- **Modal editing**: Normal, Insert, and Command modes
- **Cross-platform**: Linux, macOS, and Windows (single codebase, build tags for terminal I/O)
- **Configurable**: Line-based `vo.conf` for options and keybindings (no JSON)
- **i18n**: UI strings in English and German (configurable via `language` in config)
- **Familiar motions**: `h`/`j`/`k`/`l`, `w`/`b` (word jumps), `gg`/`G`, `0`, `dd`, etc.
- **Chords**: e.g. `jj` in Insert to return to Normal (with configurable timeout)
- **Title bar**: Configurable title and date/time format
- **Status bar**: Filename, mode, position; optional keybind feedback

---

## Requirements

- **Go 1.21+**
- Terminal with basic ANSI support (Windows: Windows Terminal or similar recommended)

---

## Build and run

```bash
git clone https://github.com/nikolaspoczekaj/vo.git
cd vo
go mod tidy
go build -o vo .
```

Windows:

```bash
go build -o vo.exe .
```

Run:

```bash
./vo                    # empty buffer
./vo path/to/file.txt   # open file (creates on first save if missing)
```

---

## Configuration (`vo.conf`)

Vo looks for `vo.conf` in the current directory at startup. The file is line-based; empty lines and lines starting with `#` are ignored.

**Options** (examples):

| Option | Example | Description |
|--------|---------|-------------|
| `timeout` | `timeout 300` | Timeout in ms for chord keys (e.g. second `j` in `jj`) |
| `relative_linenumber` | `relative_linenumber true` | Relative line numbers in the gutter |
| `indent` | `indent 4` | Spaces inserted by Tab in Insert mode |
| `language` | `language en` | UI language: `en` or `de` |
| `title` | `title vo - my editor` | Title bar text |
| `title_time_format` | `title_time_format dd.MM.yy hh:mm:ss` | Date/time format (placeholders: `dd`, `MM`, `yy`, `yyyy`, `hh`, `mm`, `ss`) |

**Keybindings:**

```
keybind <mode> <keys> <action>
```

Modes: `normal`, `insert`, `command`.  
Keys: single key (`i`, `j`), sequence (`dd`, `jj`), or special (`<Up>`, `<C-c>`).

Example:

```
keybind normal dd delete_line
keybind insert jj normal_mode
```

---

## Project structure

| Path | Purpose |
|------|---------|
| `main.go` | Entry point, CLI, wiring |
| `internal/core/` | Platform-agnostic editor logic (buffer, modes, keybinds, config, i18n) |
| `internal/terminal/` | Terminal abstraction and OS-specific implementations (Unix / Windows) |

Core is independent of OS; terminal I/O is behind the `Terminal` interface and selected by build tags (`windows` / `!windows`).

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines, code style, and how to submit changes.

---

## License

See [LICENSE](LICENSE) in this repository.
