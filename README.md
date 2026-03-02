# Vo - a vim-like editor written in Go

Vo is **inspired by** [Vim](https://www.vim.org/) and [Neovim](https://neovim.io/)—it is not meant to replace or compete with them, but to offer an alternative implementation for all operating systems while keeping a familiar feel. Vo was created with the help of AI. Contributions are welcome.


---

## Installation

### Requirements

- **Go 1.21+**
- **Git**
- Terminal with basic ANSI support

Linux:

```bash
git clone https://github.com/nikolaspoczekaj/vo.git
cd vo
go mod tidy
go build -o vo .
sudo chmod +x vo
sudo cp vo /usr/local/bin/
```

Windows:

```bash
git clone https://github.com/nikolaspoczekaj/vo.git
cd vo
go mod tidy
go build -o vo.exe .
setx PATH "%PATH%;C:\path\to\vo\"
## restart terminal

```

Run:

```bash
vo                    # empty buffer
vo path/to/file.txt   # open file (creates on first save if missing)
```

---

## Configuration (`vo.conf`)

Vo loads `vo.conf` from the system config directory at startup. If the file does not exist, it is created there with default content.

**Config location (OS-specific):**

| OS      | Path |
|---------|------|
| Linux   | `~/.config/vo/vo.conf` |
| macOS   | `~/Library/Application Support/vo/vo.conf` |
| Windows | `%APPDATA%\vo\vo.conf` (e.g. `C:\Users\<user>\AppData\Roaming\vo\vo.conf`) |

The file is line-based; empty lines and lines starting with `#` are ignored.

**Options** (examples):

| Option | Example | Description |
|--------|---------|-------------|
| `timeout` | `timeout 300` | Timeout in ms for chord keys (e.g. second `j` in `jj`) |
| `relative_linenumber` | `relative_linenumber true` | Relative line numbers in the gutter |
| `indent` | `indent 4` | Spaces inserted by Tab in Insert mode |
| `language` | `language en` | UI language: `en` or `de` |
| `title` | `title vo - my editor` | Title bar text |
| `title_time_format` | `title_time_format dd.MM.yy hh:mm:ss` | Date/time format (placeholders: `dd`, `MM`, `yy`, `yyyy`, `hh`, `mm`, `ss`) |
| `scroll_margin` | `scroll_margin 3` | Lines from top/bottom at which view scrolls (0 = only at very top/bottom) |

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
