package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/nikolaspoczekaj/vo/internal/terminal"
)

// Mode is the current editor mode.
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

// Editor ties buffer and terminal together; contains the main loop.
type Editor struct {
	Buf         *Buffer
	Term        terminal.Terminal
	Mode        Mode
	Quit        bool
	Cmd         string // current command line in Command mode
	Msg         string // status message (e.g. "Saved")
	Config      *Config
	pendingKey  rune  // for chord keys (dd in Normal, jj in Insert with timeout)
	ignoreNextJ bool  // after "jj" -> normal: ignore one extra "j" only if it comes within a short time (key repeat)
	ignoreNextJTime time.Time

	// Debounce: avoid double key when chord doesn't match (e.g. "j" then "a" for "ja").
	lastInsertKey     string
	lastInsertKeyTime time.Time

	// Show last executed keybind in status bar for a short time.
	statusKeybind     string
	statusKeybindUntil time.Time

	// scrollRow is the 0-based index of the first visible line. Updated when cursor
	// moves within scroll_margin of the top or bottom of the visible area.
	scrollRow int
}

// NewEditor creates an editor with buffer and terminal. config may be nil; then DefaultConfig() is used.
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

// Run starts the main loop (read keys, draw, react).
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
		// In Insert mode with pending prefix key (e.g. first "j" for "jj"): read with timeout.
		if e.pendingKey != 0 && e.Mode == ModeInsert && e.Config != nil {
			timeoutMs := e.Config.PendingTimeoutMs()
			key, err = e.Term.ReadKeyWithTimeout(timeoutMs)
			if err == terminal.ErrTimeout {
				// Timeout: insert first char normally, then continue
				e.Buf.InsertRune(e.pendingKey)
				e.pendingKey = 0
				continue
			}
		} else {
			// Wait up to 1 second for a key; on timeout only redraw title bar so the time updates.
			key, err = e.Term.ReadKeyWithTimeout(1000)
			if err == terminal.ErrTimeout {
				e.RedrawTitleBar()
				continue
			}
		}
		if err != nil {
			return err
		}
		e.HandleKey(key)
	}
	return nil
}

// userTimeFormatToGo converts placeholders dd.MM.yy hh:mm:ss to Go format (02.01.06 15:04:05).
// Placeholders: dd=day, MM=month, yy=2-digit year, yyyy=4-digit year, hh=hour, mm=min, ss=sec.
func userTimeFormatToGo(user string) string {
	s := user
	// Replace longest first so "yy" doesn't match inside "yyyy"
	repl := []struct{ from, to string }{
		{"yyyy", "2006"},
		{"dd", "02"},
		{"MM", "01"},
		{"yy", "06"},
		{"hh", "15"},
		{"mm", "04"},
		{"ss", "05"},
	}
	for _, r := range repl {
		s = strings.ReplaceAll(s, r.from, r.to)
	}
	return s
}

// RedrawTitleBar redraws only the title bar (row 1) with current time. Used for periodic time update when idle.
// Hides cursor during draw and restores it to the correct buffer position afterward to avoid cursor flicker.
func (e *Editor) RedrawTitleBar() {
	rows, cols, _ := e.Term.Size()
	if rows < 3 {
		rows = 25
	}
	if cols < 1 {
		cols = 80
	}
	textRows := rows - 2
	margin := 0
	if e.Config != nil {
		margin = e.Config.ScrollMargin()
	}
	maxScrollRow := 0
	if len(e.Buf.Lines) > textRows {
		maxScrollRow = len(e.Buf.Lines) - textRows
	}
	if e.scrollRow > maxScrollRow {
		e.scrollRow = maxScrollRow
	}
	if e.scrollRow < 0 {
		e.scrollRow = 0
	}
	cursorRow := e.Buf.Row
	if cursorRow <= e.scrollRow+margin {
		e.scrollRow = cursorRow - margin
		if e.scrollRow < 0 {
			e.scrollRow = 0
		}
	} else if cursorRow >= e.scrollRow+textRows-margin {
		e.scrollRow = cursorRow - textRows + 1 + margin
		if e.scrollRow > maxScrollRow {
			e.scrollRow = maxScrollRow
		}
	}
	startRow := e.scrollRow
	const lineNumWidth = 5
	const lineNumGap = 1
	contentStartCol := lineNumWidth + lineNumGap + 1
	contentWidth := cols - (lineNumWidth + lineNumGap)
	if contentWidth < 1 {
		contentWidth = 1
	}
	tabSize := 4
	if e.Config != nil {
		tabSize = e.Config.IndentSize()
	}

	move := func(row, col int) string { return fmt.Sprintf("\x1b[%d;%dH", row, col) }
	const hideCursor = "\x1b[?25l"
	const showCursor = "\x1b[?25h"
	const cursorBlock = "\x1b[1 q"
	const cursorBar = "\x1b[5 q"
	const titleBarOn = "\x1b[100m\x1b[97m"
	const statusBarOff = "\x1b[0m"
	const clearToEnd = "\x1b[K"

	titleCols := cols
	if titleCols > 0 {
		titleCols--
	}
	titleText := "vo - a vim-like editor"
	timeStr := ""
	if e.Config != nil {
		titleText = e.Config.Title()
		if userFmt := e.Config.TitleTimeFormat(); userFmt != "" {
			goFmt := userTimeFormatToGo(userFmt)
			timeStr = time.Now().Format(goFmt)
		}
	}
	var sb strings.Builder
	sb.WriteString(hideCursor)
	sb.WriteString(move(1, 1))
	sb.WriteString(titleBarOn)
	if timeStr != "" {
		if len(titleText)+1+len(timeStr) <= titleCols {
			sb.WriteString(titleText)
			sb.WriteString(strings.Repeat(" ", titleCols-len(titleText)-len(timeStr)))
			sb.WriteString(timeStr)
		} else {
			trunc := titleText
			if len(trunc) > titleCols-len(timeStr)-1 {
				trunc = trunc[:titleCols-len(timeStr)-1]
			}
			sb.WriteString(trunc)
			sb.WriteString(strings.Repeat(" ", titleCols-len(trunc)-len(timeStr)))
			sb.WriteString(timeStr)
		}
	} else {
		sb.WriteString(titleText)
		for i := len(titleText); i < titleCols; i++ {
			sb.WriteByte(' ')
		}
	}
	for i := titleCols; i < cols; i++ {
		sb.WriteByte(' ')
	}
	sb.WriteString(statusBarOff)
	sb.WriteString(clearToEnd)

	curRow := e.Buf.Row - startRow + 2
	curCol := contentStartCol + byteOffsetToDisplayCol(e.Buf.CurrentLine(), e.Buf.Col, tabSize)
	if curCol > contentStartCol+contentWidth-1 {
		curCol = contentStartCol + contentWidth - 1
	}
	if curRow >= 2 && curRow <= rows-1 {
		sb.WriteString(move(curRow, curCol))
		if e.Mode == ModeInsert {
			sb.WriteString(cursorBar)
		} else {
			sb.WriteString(cursorBlock)
		}
	}
	sb.WriteString(showCursor)
	e.Term.Write(sb.String())
	e.Term.Flush()
}

// Redraw paints the visible area, title bar, and status line. Cursor is hidden while drawing; output is
// written in one go and flushed once to avoid flicker.
func (e *Editor) Redraw() {
	rows, cols, _ := e.Term.Size()
	if rows < 3 {
		rows = 25
	}
	if cols < 1 {
		cols = 80
	}
	textRows := rows - 2 // one row title bar, one row status bar
	e.Buf.ClampCursor()

	// Update scroll position: scroll down when cursor is within margin lines of bottom,
	// scroll up when cursor is within margin lines of top. Otherwise keep scroll stable.
	margin := 0
	if e.Config != nil {
		margin = e.Config.ScrollMargin()
	}
	maxScrollRow := 0
	if len(e.Buf.Lines) > textRows {
		maxScrollRow = len(e.Buf.Lines) - textRows
	}
	if e.scrollRow > maxScrollRow {
		e.scrollRow = maxScrollRow
	}
	if e.scrollRow < 0 {
		e.scrollRow = 0
	}
	cursorRow := e.Buf.Row
	if cursorRow <= e.scrollRow+margin {
		// Cursor at or above the "scroll up" line → scroll up so cursor is margin lines from top
		e.scrollRow = cursorRow - margin
		if e.scrollRow < 0 {
			e.scrollRow = 0
		}
	} else if cursorRow >= e.scrollRow+textRows-margin {
		// Cursor at or below the "scroll down" line → scroll down so cursor is margin lines from bottom
		e.scrollRow = cursorRow - textRows + 1 + margin
		if e.scrollRow > maxScrollRow {
			e.scrollRow = maxScrollRow
		}
	}
	startRow := e.scrollRow
	visible := e.Buf.VisibleLines(startRow, textRows)

	const (
		hideCursor     = "\x1b[?25l"
		showCursor     = "\x1b[?25h"
		cursorBlock    = "\x1b[1 q"  // blinking block (full cell) for Normal/Command
		cursorBar      = "\x1b[5 q"  // blinking bar (thin) for Insert
		clearToEnd     = "\x1b[K"
		titleBarOn     = "\x1b[100m\x1b[97m" // same style as status bar (dark gray bg, white text)
		statusBarOn    = "\x1b[100m\x1b[97m"
		statusBarOff   = "\x1b[0m"
		lineNumStyle   = "\x1b[90m" // dim gray for line numbers
		lineNumStyleOff = "\x1b[0m"
	)
	const lineNumWidth = 5
	const lineNumGap  = 1   // gap between number and line content
	contentStartCol  := lineNumWidth + lineNumGap + 1
	contentWidth     := cols - (lineNumWidth + lineNumGap)
	if contentWidth < 1 {
		contentWidth = 1
	}
	relativeNum := e.Config != nil && e.Config.RelativeLineNumber()
	tabSize := 4
	if e.Config != nil {
		tabSize = e.Config.IndentSize()
	}
	move := func(row, col int) string { return fmt.Sprintf("\x1b[%d;%dH", row, col) }

	var sb strings.Builder
	sb.Grow(256 * (textRows + 3))
	sb.WriteString(hideCursor)

	// Title bar (row 1): title left, date/time right. Use titleCols = cols-1 so the last column is not written (avoids cutoff on some terminals).
	titleCols := cols
	if titleCols > 0 {
		titleCols--
	}
	sb.WriteString(move(1, 1))
	sb.WriteString(titleBarOn)
	titleText := "vo - a vim-like editor"
	timeStr := ""
	if e.Config != nil {
		titleText = e.Config.Title()
		if userFmt := e.Config.TitleTimeFormat(); userFmt != "" {
			goFmt := userTimeFormatToGo(userFmt)
			timeStr = time.Now().Format(goFmt)
		}
	}
	if timeStr != "" {
		if len(titleText)+1+len(timeStr) <= titleCols {
			sb.WriteString(titleText)
			sb.WriteString(strings.Repeat(" ", titleCols-len(titleText)-len(timeStr)))
			sb.WriteString(timeStr)
		} else {
			trunc := titleText
			if len(trunc) > titleCols-len(timeStr)-1 {
				trunc = trunc[:titleCols-len(timeStr)-1]
			}
			sb.WriteString(trunc)
			sb.WriteString(strings.Repeat(" ", titleCols-len(trunc)-len(timeStr)))
			sb.WriteString(timeStr)
		}
	} else {
		sb.WriteString(titleText)
		for i := len(titleText); i < titleCols; i++ {
			sb.WriteByte(' ')
		}
	}
	for i := titleCols; i < cols; i++ {
		sb.WriteByte(' ')
	}
	sb.WriteString(statusBarOff)
	sb.WriteString(clearToEnd)

	for i := 0; i < textRows; i++ {
		sb.WriteString(move(i+2, 1))
		lineRow := startRow + i
		var num int
		if relativeNum {
			if lineRow == e.Buf.Row {
				num = lineRow + 1
			} else {
				n := e.Buf.Row - lineRow
				if n < 0 {
					n = -n
				}
				num = n
			}
		} else {
			num = lineRow + 1
		}
		sb.WriteString(lineNumStyle)
		sb.WriteString(fmt.Sprintf("%*d", lineNumWidth, num))
		sb.WriteString(lineNumStyleOff)
		sb.WriteString(" ")
		sb.WriteString(move(i+2, contentStartCol))
		if i < len(visible) {
			line := visible[i]
			line = expandTabs(line, tabSize)
			if len(line) > contentWidth {
				line = line[:contentWidth]
			}
			sb.WriteString(line)
		}
		sb.WriteString(clearToEnd)
	}

	sb.WriteString(move(rows, 1))
	sb.WriteString(statusBarOn)
	status := e.statusText()
	// If keybind was just executed, show it right-aligned so main content stays on the left
	if e.statusKeybind != "" && time.Now().Before(e.statusKeybindUntil) {
		rightPart := e.statusKeybind
		if len(status)+1+len(rightPart) <= cols {
			status = status + strings.Repeat(" ", cols-len(status)-len(rightPart)) + rightPart
		} else {
			maxLeft := cols - len(rightPart) - 1
			if maxLeft > 0 && len(status) > maxLeft {
				status = status[:maxLeft]
			}
			status = status + strings.Repeat(" ", cols-len(status)-len(rightPart)) + rightPart
		}
	}
	if len(status) > cols {
		status = status[:cols]
	}
	sb.WriteString(status)
	// Fill rest of line with background color to the right edge
	for i := len(status); i < cols; i++ {
		sb.WriteByte(' ')
	}
	sb.WriteString(statusBarOff)
	sb.WriteString(clearToEnd)

	curRow := e.Buf.Row - startRow + 2 // +2 because row 1 is title bar
	curCol := contentStartCol + byteOffsetToDisplayCol(e.Buf.CurrentLine(), e.Buf.Col, tabSize)
	if curCol > contentStartCol+contentWidth-1 {
		curCol = contentStartCol + contentWidth - 1
	}
	if curRow >= 2 && curRow <= rows-1 {
		sb.WriteString(move(curRow, curCol))
		// Cursor shape: block in Normal/Command, thin bar in Insert
		if e.Mode == ModeInsert {
			sb.WriteString(cursorBar)
		} else {
			sb.WriteString(cursorBlock)
		}
	}
	sb.WriteString(showCursor)

	e.Term.Write(sb.String())
	e.Term.Flush()
}

// expandTabs replaces tabs with spaces (tabSize columns per tab).
func expandTabs(s string, tabSize int) string {
	if tabSize <= 0 {
		tabSize = 4
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabSize - (col % tabSize)
			b.Grow(n)
			for i := 0; i < n; i++ {
				b.WriteByte(' ')
			}
			col += n
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// byteOffsetToDisplayCol returns the display column (0-based) for the byte offset in s (tabs expanded).
func byteOffsetToDisplayCol(s string, byteOff int, tabSize int) int {
	if tabSize <= 0 {
		tabSize = 4
	}
	col := 0
	for i, r := range s {
		if i >= byteOff {
			return col
		}
		if r == '\t' {
			col += tabSize - (col % tabSize)
		} else {
			col++
		}
	}
	return col
}

func (e *Editor) statusText() string {
	// Clear keybind display when expired
	if e.statusKeybind != "" && !time.Now().Before(e.statusKeybindUntil) {
		e.statusKeybind = ""
	}
	if e.Msg != "" {
		return e.Msg
	}
	if e.Mode == ModeCommand {
		return ":" + e.Cmd
	}
	lang := LangEN
	if e.Config != nil {
		lang = e.Config.Language()
	}
	modeKey := "mode_normal"
	switch e.Mode {
	case ModeInsert:
		modeKey = "mode_insert"
	case ModeCommand:
		modeKey = "mode_command"
	}
	return e.Buf.StatusLine(lang, T(lang, modeKey))
}

// HandleKey processes a key according to the current mode.
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

	// Ignore extra "j" after "jj" only if it arrives within a short window (key repeat); otherwise treat as move_down.
	const ignoreJWindowMs = 80
	if e.ignoreNextJ && keyStr == "j" && time.Since(e.ignoreNextJTime) < ignoreJWindowMs*time.Millisecond {
		e.ignoreNextJ = false
		return
	}
	e.ignoreNextJ = false

	// Config-based: chord keys (e.g. dd) and lookup
	if e.Config != nil && e.Config.Keybinds != nil {
		if e.pendingKey != 0 {
			compound := string(e.pendingKey) + keyStr
			e.pendingKey = 0
			if action := e.Config.Keybinds.Action("normal", compound); action != "" {
				e.setStatusKeybind(compound, action)
				e.runAction(action)
				return
			}
		}
		if action := e.Config.Keybinds.Action("normal", keyStr); action != "" {
			e.setStatusKeybind(keyStr, action)
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

// setStatusKeybind shows the executed keybind in the status bar for a short time.
func (e *Editor) setStatusKeybind(keys, action string) {
	e.statusKeybind = keys + " → " + action
	e.statusKeybindUntil = time.Now().Add(1500 * time.Millisecond)
}

// runAction runs the action named by the keybind config.
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
		e.ignoreNextJTime = time.Now()
	}
}

func (e *Editor) handleInsertKey(k terminal.Key) {
	keyStr := k.ConfigString()
	if keyStr == "" {
		return
	}

	// Debounce: if this key was just inserted as the second key of a non-matching chord, skip duplicate (e.g. "j" then "a" -> "ja" not "jaa").
	const debounceMs = 120
	if e.lastInsertKey != "" && keyStr == e.lastInsertKey && time.Since(e.lastInsertKeyTime) < debounceMs*time.Millisecond {
		e.lastInsertKey = ""
		return
	}
	e.lastInsertKey = ""

	if e.Config != nil && e.Config.Keybinds != nil {
		if e.pendingKey != 0 {
			compound := string(e.pendingKey) + keyStr
			saved := e.pendingKey
			e.pendingKey = 0
			if action := e.Config.Keybinds.Action("insert", compound); action != "" {
				e.setStatusKeybind(compound, action)
				e.runAction(action)
				return
			}
			// No binding for chord: insert first char and current key, then return to avoid double processing
			e.Buf.InsertRune(saved)
			e.Buf.InsertRune(k.Rune)
			e.lastInsertKey = keyStr
			e.lastInsertKeyTime = time.Now()
			return
		}
		if e.pendingKey == 0 {
			if action := e.Config.Keybinds.Action("insert", keyStr); action != "" {
				e.setStatusKeybind(keyStr, action)
				e.runAction(action)
				return
			}
			if e.Config.Keybinds.IsPrefix("insert", keyStr) {
				e.pendingKey = rune(keyStr[0])
				return
			}
		}
	}

	// Fallback: special keys and insert rune
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
	case k.Rune == '\t' || k.Rune == terminal.KeyRuneTab:
		n := 4
		if e.Config != nil {
			n = e.Config.IndentSize()
		}
		e.Buf.InsertSpaces(n)
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
	lang := LangEN
	if e.Config != nil {
		lang = e.Config.Language()
	}
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "q", "quit":
		if e.Buf.Dirty {
			e.Msg = T(lang, "msg_unsaved")
			return
		}
		e.Quit = true
	case "q!":
		e.Quit = true
	case "w", "write":
		if err := e.Buf.Save(); err != nil {
			e.Msg = fmt.Sprintf(T(lang, "msg_error"), err.Error())
			return
		}
		e.Msg = T(lang, "msg_saved")
	case "wq":
		if err := e.Buf.Save(); err != nil {
			e.Msg = fmt.Sprintf(T(lang, "msg_error"), err.Error())
			return
		}
		e.Msg = T(lang, "msg_saved")
		e.Quit = true
	default:
		e.Msg = fmt.Sprintf(T(lang, "msg_unknown_cmd"), parts[0])
	}
}
