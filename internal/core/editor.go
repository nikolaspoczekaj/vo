package core

import (
	"strings"

	"nim/internal/terminal"
)

// Mode ist der aktuelle Editor-Modus.
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
)

func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeCommand:
		return "COMMAND"
	default:
		return "?"
	}
}

// Editor verbindet Buffer und Terminal; enthält die Hauptschleife.
type Editor struct {
	Buf         *Buffer
	Term        terminal.Terminal
	Mode        Mode
	Quit        bool
	Cmd         string // aktuelle Befehlszeile im Command-Modus
	Msg         string // Statusmeldung (z. B. "Gespeichert")
	Config      *Config
	pendingKey  rune  // für Mehrfach-Tasten (dd im Normal-Modus, jj im Insert mit Timeout)
	ignoreNextJ bool  // nach "jj" → normal: ein überzähliges "j" (Tastenwiederholung) ignorieren
}

// NewEditor erstellt einen Editor mit Buffer und Terminal.
// config kann nil sein, dann wird DefaultConfig() verwendet.
func NewEditor(buf *Buffer, term terminal.Terminal, config *Config) *Editor {
	if config == nil {
		config = DefaultConfig()
	}
	return &Editor{
		Buf:  buf,
		Term: term,
		Mode: ModeNormal,
		Config: config,
	}
}

// Run startet die Hauptschleife (Tasten lesen, zeichnen, reagieren).
func (e *Editor) Run() error {
	if err := e.Term.Init(); err != nil {
		return err
	}
	defer e.Term.Close()
	defer e.Term.ShowCursor()

	for !e.Quit {
		e.Redraw()
		var key terminal.Key
		var err error
		// Im Insert-Modus mit wartendem Präfix-Key (z. B. erstes "j" für "jj"): mit Timeout lesen.
		if e.pendingKey != 0 && e.Mode == ModeInsert && e.Config != nil {
			timeoutMs := e.Config.PendingTimeoutMs()
			key, err = e.Term.ReadKeyWithTimeout(timeoutMs)
			if err == terminal.ErrTimeout {
				// Timeout: erstes Zeichen normal einfügen, dann weiter
				e.Buf.InsertRune(e.pendingKey)
				e.pendingKey = 0
				continue
			}
		} else {
			key, err = e.Term.ReadKey()
		}
		if err != nil {
			return err
		}
		e.HandleKey(key)
	}
	return nil
}

// Redraw zeichnet den sichtbaren Bereich und die Statuszeile.
// Inkrementell ohne volles ClearScreen, um Flackern zu vermeiden.
func (e *Editor) Redraw() {
	rows, cols, _ := e.Term.Size()
	if rows < 2 {
		rows = 24
	}
	if cols < 1 {
		cols = 80
	}
	textRows := rows - 1
	e.Buf.ClampCursor()
	startRow := 0
	if e.Buf.Row >= textRows {
		startRow = e.Buf.Row - textRows + 1
	}
	visible := e.Buf.VisibleLines(startRow, textRows)

	// Nur Zeilen überschreiben + Rest der Zeile löschen (kein volles ClearScreen)
	const clearToEnd = "\x1b[K"
	for i := 0; i < textRows; i++ {
		e.Term.MoveCursor(i+1, 1)
		if i < len(visible) {
			line := visible[i]
			if len(line) > cols {
				line = line[:cols]
			}
			e.Term.Write(line)
		}
		e.Term.Write(clearToEnd)
	}
	// Statuszeile
	e.Term.MoveCursor(rows, 1)
	status := e.statusText()
	if len(status) > cols {
		status = status[:cols]
	}
	e.Term.Write(status)
	e.Term.Write(clearToEnd)
	// Cursor setzen
	curRow := e.Buf.Row - startRow + 1
	curCol := e.Buf.Col + 1
	if curRow >= 1 && curRow <= rows {
		e.Term.MoveCursor(curRow, curCol)
	}
	e.Term.Flush()
}

func (e *Editor) statusText() string {
	if e.Msg != "" {
		return e.Msg
	}
	switch e.Mode {
	case ModeCommand:
		return ":" + e.Cmd
	default:
		return e.Buf.StatusLine(e.Mode.String())
	}
}

// HandleKey verarbeitet eine Taste je nach Modus.
func (e *Editor) HandleKey(k terminal.Key) {
	switch e.Mode {
	case ModeNormal:
		e.handleNormalKey(k)
	case ModeInsert:
		e.handleInsertKey(k)
	case ModeCommand:
		e.handleCommandKey(k)
	}
}

func (e *Editor) handleNormalKey(k terminal.Key) {
	e.Msg = ""

	keyStr := k.ConfigString()
	if keyStr == "" {
		return
	}

	// Ein überzähliges "j" nach "jj" (Tastenwiederholung) ignorieren
	if e.ignoreNextJ && keyStr == "j" {
		e.ignoreNextJ = false
		return
	}
	e.ignoreNextJ = false

	// Config-basiert: Mehrfach-Tasten (z. B. dd) und Lookup
	if e.Config != nil && e.Config.Keybinds != nil {
		if e.pendingKey != 0 {
			compound := string(e.pendingKey) + keyStr
			e.pendingKey = 0
			if action := e.Config.Keybinds.Action("normal", compound); action != "" {
				e.runAction(action)
				return
			}
		}
		if action := e.Config.Keybinds.Action("normal", keyStr); action != "" {
			e.runAction(action)
			return
		}
		if e.Config.Keybinds.IsPrefix("normal", keyStr) {
			e.pendingKey = rune(keyStr[0])
			return
		}
		return
	}

	// Fallback ohne Config (eingebaute Keybinds)
	switch {
	case k.Esc:
	case k.Enter:
		e.Buf.InsertRune('\n')
	case k.Up, k.Rune == 'k':
		e.Buf.MoveUp()
	case k.Down, k.Rune == 'j':
		e.Buf.MoveDown()
	case k.Left, k.Rune == 'h':
		e.Buf.MoveLeft()
	case k.Right, k.Rune == 'l':
		e.Buf.MoveRight()
	case k.Home:
		e.Buf.MoveLineStart()
	case k.End:
		e.Buf.MoveLineEnd()
	case k.Backspace:
		e.Buf.DeleteRuneBackspace()
	case k.Rune == 'i':
		e.Mode = ModeInsert
	case k.Rune == 'a':
		e.Buf.MoveRight()
		e.Mode = ModeInsert
	case k.Rune == 'A':
		e.Buf.MoveLineEnd()
		e.Mode = ModeInsert
	case k.Rune == 'o':
		e.Buf.MoveLineEnd()
		e.Buf.InsertRune('\n')
		e.Mode = ModeInsert
	case k.Rune == 'O':
		e.Buf.MoveLineStart()
		e.Buf.InsertRune('\n')
		e.Buf.MoveUp()
		e.Mode = ModeInsert
	case k.Rune == ':':
		e.Mode = ModeCommand
		e.Cmd = ""
	case k.Ctrl && k.Rune == 'c':
		e.Quit = true
	}
}

// runAction führt eine durch die Keybind-Config benannte Aktion aus.
func (e *Editor) runAction(action string) {
	switch action {
	case "move_left":
		e.Buf.MoveLeft()
	case "move_right":
		e.Buf.MoveRight()
	case "move_up":
		e.Buf.MoveUp()
	case "move_down":
		e.Buf.MoveDown()
	case "move_line_start":
		e.Buf.MoveLineStart()
	case "move_line_end":
		e.Buf.MoveLineEnd()
	case "buffer_start":
		e.Buf.MoveBufferStart()
	case "buffer_end":
		e.Buf.MoveBufferEnd()
	case "next_word":
		e.Buf.MoveToNextWord()
	case "prev_word":
		e.Buf.MoveToPrevWord()
	case "split_line":
		e.Buf.InsertRune('\n')
	case "delete_backspace":
		e.Buf.DeleteRuneBackspace()
	case "delete_line":
		e.Buf.DeleteLine()
	case "insert":
		e.Mode = ModeInsert
	case "insert_after":
		e.Buf.MoveRight()
		e.Mode = ModeInsert
	case "insert_at_line_end":
		e.Buf.MoveLineEnd()
		e.Mode = ModeInsert
	case "open_line_below":
		e.Buf.MoveLineEnd()
		e.Buf.InsertRune('\n')
		e.Mode = ModeInsert
	case "open_line_above":
		e.Buf.MoveLineStart()
		e.Buf.InsertRune('\n')
		e.Buf.MoveUp()
		e.Mode = ModeInsert
	case "command_mode":
		e.Mode = ModeCommand
		e.Cmd = ""
	case "quit":
		e.Quit = true
	case "normal_mode":
		e.Mode = ModeNormal
		e.pendingKey = 0
		e.ignoreNextJ = true
	}
}

func (e *Editor) handleInsertKey(k terminal.Key) {
	keyStr := k.ConfigString()
	if keyStr == "" {
		return
	}

	if e.Config != nil && e.Config.Keybinds != nil {
		if e.pendingKey != 0 {
			compound := string(e.pendingKey) + keyStr
			saved := e.pendingKey
			e.pendingKey = 0
			if action := e.Config.Keybinds.Action("insert", compound); action != "" {
				e.runAction(action)
				return
			}
			// Keine Bindung für compound: erstes Zeichen einfügen, dann aktuelle Taste verarbeiten
			e.Buf.InsertRune(saved)
		}
		if e.pendingKey == 0 {
			if action := e.Config.Keybinds.Action("insert", keyStr); action != "" {
				e.runAction(action)
				return
			}
			if e.Config.Keybinds.IsPrefix("insert", keyStr) {
				e.pendingKey = rune(keyStr[0])
				return
			}
		}
	}

	// Fallback: Sondertasten und Zeichen einfügen
	switch {
	case k.Esc:
		e.Mode = ModeNormal
		e.pendingKey = 0
	case k.Enter:
		e.Buf.InsertRune('\n')
	case k.Backspace:
		e.Buf.DeleteRuneBackspace()
	case k.Left:
		e.Buf.MoveLeft()
	case k.Right:
		e.Buf.MoveRight()
	case k.Up:
		e.Buf.MoveUp()
	case k.Down:
		e.Buf.MoveDown()
	case k.Home:
		e.Buf.MoveLineStart()
	case k.End:
		e.Buf.MoveLineEnd()
	case k.IsRune() && k.Rune != 0:
		e.Buf.InsertRune(k.Rune)
	}
}

func (e *Editor) handleCommandKey(k terminal.Key) {
	switch {
	case k.Enter:
		e.executeCommand()
		e.Mode = ModeNormal
		e.Cmd = ""
	case k.Esc:
		e.Mode = ModeNormal
		e.Cmd = ""
	case k.Backspace:
		if len(e.Cmd) > 0 {
			e.Cmd = e.Cmd[:len(e.Cmd)-1]
		}
	case k.IsRune() && k.Rune != 0:
		e.Cmd += string(k.Rune)
	}
}

func (e *Editor) executeCommand() {
	cmd := strings.TrimSpace(e.Cmd)
	if cmd == "" {
		return
	}
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "q", "quit":
		if e.Buf.Dirty {
			e.Msg = "Ungespeicherte Änderungen ( :w zum Speichern, :q! zum Verlassen ohne Speichern )"
			return
		}
		e.Quit = true
	case "q!":
		e.Quit = true
	case "w", "write":
		if err := e.Buf.Save(); err != nil {
			e.Msg = "Fehler: " + err.Error()
			return
		}
		e.Msg = "Gespeichert"
	case "wq":
		if err := e.Buf.Save(); err != nil {
			e.Msg = "Fehler: " + err.Error()
			return
		}
		e.Msg = "Gespeichert"
		e.Quit = true
	default:
		e.Msg = "Unbekannter Befehl: " + parts[0]
	}
}
