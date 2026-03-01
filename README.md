# Nim – Vim-ähnlicher Editor in Go

Minimaler Terminal-Editor für Linux und Windows mit klarer Trennung von Core-Logik und betriebssystemspezifischem Code.

## Projektstruktur

- **`main.go`** – Einstiegspunkt, CLI `nim [dateiname]`
- **`internal/core/`** – plattformunabhängige Editor-Logik
  - `buffer.go` – Textpuffer, Laden/Speichern, Cursor, Einfügen/Löschen
  - `editor.go` – Modi (Normal/Insert/Command), Hauptschleife, Tastenbehandlung
- **`internal/terminal/`** – Terminal-Zugriff
  - `terminal.go` – gemeinsame Schnittstelle `Terminal`
  - `keys.go`, `parser.go` – Tasten-Definitionen und ANSI-Parser (plattformübergreifend)
  - `terminal_unix.go` – Linux/macOS (Build-Tag: `!windows`)
  - `terminal_windows.go` – Windows (Build-Tag: `windows`)

## Bauen und Ausführen

```bash
go mod tidy
go build -o nim .
# bzw. unter Windows: nim.exe
```

Erste Datei bearbeiten:

```bash
nim dateiname
```

Falls die Datei nicht existiert, wird ein leerer Buffer geöffnet; beim ersten Speichern wird sie angelegt.

## Erster Meilenstein: Einfaches Bearbeiten

- **`nim dateiname`** – Datei öffnen (oder leeren Buffer)
- **Pfeiltasten** – Cursor bewegen
- **`i`** – Insert-Modus (Text einfügen)
- **`Esc`** – zurück in den Normal-Modus
- **`a`** – Einfügen nach Cursor, **`A`** – Einfügen am Zeilenende
- **`o`** / **`O`** – neue Zeile unter/über der aktuellen
- **`:w`** – speichern, **`:q`** – beenden, **`:wq`** – speichern und beenden, **`:q!`** – beenden ohne zu speichern
- **Strg+C** – sofort beenden

## Konfiguration (nim.conf)

Die Konfiguration wird aus **`nim.conf`** (im aktuellen Verzeichnis) **zeilenweise** gelesen und **einmal beim Start** in den Speicher geladen. Kein festes Dateiformat; Leerzeilen und Zeilen, die mit `#` beginnen, werden ignoriert.

**Optionen** (z. B. Timeout für Doppel-Tasten):

- `timeout 300` oder `timeout = 300` – Wartezeit in Millisekunden, bis ein zweiter Tastendruck (z. B. zweites **j** bei **jj**) nicht mehr als Tastenfolge gewertet wird. Standard: 300.
- `relative_linenumber true` bzw. `false` – Bei `true` zeigen die Zeilennummern links den Abstand zur aktuellen Zeile (wie in Neovim), bei `false` die absolute Zeilennummer. Standard: false.

**Keybinds:**

- `keybind <mode> <keys> <action>`
- Modus: `normal`, `insert`, `command`
- Keys: Einzelzeichen (`i`, `j`), Folgen (`dd`, `jj`) oder Sonderzeichen (`<Up>`, `<C-c>`)

Beispielzeilen:

```
timeout 300
keybind normal dd delete_line
keybind insert jj normal_mode
```

Im **Insert-Modus** kann **jj** wie Esc wirken (Wechsel in den Normal-Modus). Wird **j** nur einmal gedrückt oder kommt die zweite Taste nach Ablauf des Timeouts, wird **j** normal als Zeichen eingefügt.

Sonderzeichen: `<Up>`, `<Down>`, `<Left>`, `<Right>`, `<Enter>`, `<Backspace>`, `<Esc>`, `<Home>`, `<End>`, `<C-x>` (Strg+x).

Aktionen (u. a.): `move_left`, `move_right`, `move_up`, `move_down`, `move_line_start`, `move_line_end`, `split_line`, `delete_backspace`, `delete_line`, `insert`, `insert_after`, `insert_at_line_end`, `open_line_below`, `open_line_above`, `command_mode`, `quit`, `normal_mode` (nur Insert).

## Abhängigkeiten

- Go 1.21+
- `golang.org/x/term` (Raw-Mode und Terminalgröße auf beiden Plattformen)