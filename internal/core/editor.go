package core

import (
	"fmt"
	"strconv"
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

// PopupKind is the type of popup notification (info, error, etc.).
type PopupKind int

const (
	PopupInfo PopupKind = iota
	PopupError
)

// popupEntry holds a single popup message and when it expires.
type popupEntry struct {
	Text string
	Kind PopupKind
	Until time.Time
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
	// scrollCol is the display column (0-based) of the left edge of the content area; used for horizontal scrolling of long lines.
	scrollCol int

	// countPrefix is the numeric prefix for the next command (e.g. 4 for "4j"). 0 = none.
	countPrefix int

	// popups are notifications shown as small boxes in the corner; expired entries are removed on each Redraw.
	popups []popupEntry
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

// ShowPopup adds a notification (info or error) that is shown for the configured duration (popup_timeout in vo.conf).
// Can be called from anywhere that has access to the editor, e.g. after save errors or other feedback.
func (e *Editor) ShowPopup(text string, kind PopupKind) {
	if e == nil {
		return
	}
	timeoutSec := 3
	if e.Config != nil {
		timeoutSec = e.Config.PopupTimeoutSec()
	}
	e.popups = append(e.popups, popupEntry{
		Text: text,
		Kind: kind,
		Until: time.Now().Add(time.Duration(timeoutSec) * time.Second),
	})
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
	const lineNumGap = 2
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
	cursorDisplayCol := byteOffsetToDisplayCol(e.Buf.CurrentLine(), e.Buf.Col, tabSize)
	curCol := contentStartCol + (cursorDisplayCol - e.scrollCol)
	if curCol < contentStartCol {
		curCol = contentStartCol
	}
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
	const lineNumGap  = 2   // gap between number and line content
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

	// Update horizontal scroll so cursor stays visible in the content area.
	currentExpanded := expandTabs(e.Buf.CurrentLine(), tabSize)
	cursorDisplayCol := byteOffsetToDisplayCol(e.Buf.CurrentLine(), e.Buf.Col, tabSize)
	lineWidth := len([]rune(currentExpanded))
	if cursorDisplayCol < e.scrollCol {
		e.scrollCol = cursorDisplayCol
	}
	if cursorDisplayCol >= e.scrollCol+contentWidth {
		e.scrollCol = cursorDisplayCol - contentWidth + 1
	}
	if e.scrollCol < 0 {
		e.scrollCol = 0
	}
	maxScrollCol := lineWidth - contentWidth
	if maxScrollCol < 0 {
		maxScrollCol = 0
	}
	if e.scrollCol > maxScrollCol {
		e.scrollCol = maxScrollCol
	}

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
			expanded := expandTabs(line, tabSize)
			line = visibleLineSlice(expanded, e.scrollCol, contentWidth)
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

	// Popups: bottom-right, one empty row above status bar.
	// Prune expired, then draw newest first so the newest popup is closest to the bottom.
	now := time.Now()
	n := 0
	for i := range e.popups {
		if now.Before(e.popups[i].Until) {
			e.popups[n] = e.popups[i]
			n++
		}
	}
	e.popups = e.popups[:n]
	const popupWidth = 42
	const popupMaxLines = 2
	const popupMaxCount = 5
	const (
		popupStyleInfo  = "\x1b[44m\x1b[97m" // blue bg, white text
		popupStyleError = "\x1b[41m\x1b[97m" // red bg, white text
		popupStyleOff   = "\x1b[0m"
	)
	// Start drawing from the bottom (rows-2), leaving one empty line between popup boxes and the status bar (last row).
	popupRow := rows - 2
	showFrom := len(e.popups) - popupMaxCount
	if showFrom < 0 {
		showFrom = 0
	}
	// Column where the popup box starts so that it has a fixed width.
	popupStartCol := cols - popupWidth + 1
	if popupStartCol < 1 {
		popupStartCol = 1
	}
	for i := len(e.popups) - 1; i >= showFrom && popupRow >= 2; i-- {
		p := &e.popups[i]
		style := popupStyleInfo
		if p.Kind == PopupError {
			style = popupStyleError
		}
		lines := popupTextToLines(p.Text, popupWidth, popupMaxLines)
		// Draw each popup from bottom to top so multiple popups stack upwards from the corner.
		for l := len(lines) - 1; l >= 0 && popupRow >= 2; l-- {
			line := lines[l]
			runes := []rune(line)
			if len(runes) > popupWidth {
				runes = runes[:popupWidth]
			}
			// Pad to fixed width so the box always has the same size.
			if len(runes) < popupWidth {
				line = string(runes) + strings.Repeat(" ", popupWidth-len(runes))
			} else {
				line = string(runes)
			}
			if popupRow >= rows {
				break
			}
			sb.WriteString(move(popupRow, popupStartCol))
			sb.WriteString(style)
			sb.WriteString(line)
			sb.WriteString(popupStyleOff)
			sb.WriteString(clearToEnd)
			popupRow--
		}
	}

	curRow := e.Buf.Row - startRow + 2 // +2 because row 1 is title bar
	curCol := contentStartCol + (cursorDisplayCol - e.scrollCol)
	if curCol < contentStartCol {
		curCol = contentStartCol
	}
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

// popupTextToLines splits text by newlines, truncates lines to maxWidth runes, returns at most maxLines lines.
func popupTextToLines(text string, maxWidth, maxLines int) []string {
	var out []string
	for _, s := range strings.Split(text, "\n") {
		s = strings.TrimSpace(s)
		runes := []rune(s)
		for len(out) < maxLines && len(runes) > 0 {
			if len(runes) <= maxWidth {
				out = append(out, string(runes))
				break
			}
			out = append(out, string(runes[:maxWidth]))
			runes = runes[maxWidth:]
		}
		if len(out) >= maxLines {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	return out
}

// visibleLineSlice returns the substring of expanded (tabs already expanded) that occupies the display columns [scrollCol, scrollCol+width). One rune = one column.
func visibleLineSlice(expanded string, scrollCol, width int) string {
	runes := []rune(expanded)
	if scrollCol >= len(runes) {
		return ""
	}
	start := scrollCol
	end := start + width
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
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

	// Numeric prefix: 1-9 start or extend count; 0 extends if count > 0, else runs move_line_start.
	if k.Rune >= '1' && k.Rune <= '9' {
		if e.countPrefix == 0 {
			e.countPrefix = int(k.Rune - '0')
		} else {
			e.countPrefix = e.countPrefix*10 + int(k.Rune-'0')
			if e.countPrefix > 9999 {
				e.countPrefix = 9999
			}
		}
		return
	}
	if k.Rune == '0' {
		if e.countPrefix > 0 {
			e.countPrefix *= 10
			if e.countPrefix > 9999 {
				e.countPrefix = 9999
			}
			return
		}
		// "0" without prefix = move line start
		e.runActionWithCount("move_line_start", 1)
		return
	}

	// Count to apply to the next command (1 if none was given).
	n := e.countPrefix
	if n == 0 {
		n = 1
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
				e.runActionWithCount(action, n)
				e.countPrefix = 0
				return
			}
			// Chord not bound: fall through and try keyStr as single key (count still applies)
		}
		if action := e.Config.Keybinds.Action("normal", keyStr); action != "" {
			e.setStatusKeybind(keyStr, action)
			e.runActionWithCount(action, n)
			e.countPrefix = 0
			return
		}
		if e.Config.Keybinds.IsPrefix("normal", keyStr) {
			e.pendingKey = rune(keyStr[0])
			return
		}
		e.countPrefix = 0
		return
	}

	// Fallback ohne Config (eingebaute Keybinds)
	e.countPrefix = 0
	switch {
	case k.Esc:
	case k.Enter:
		for i := 0; i < n; i++ {
			e.Buf.InsertRune('\n')
		}
	case k.Up, k.Rune == 'k':
		for i := 0; i < n; i++ {
			e.Buf.MoveUp()
		}
	case k.Down, k.Rune == 'j':
		for i := 0; i < n; i++ {
			e.Buf.MoveDown()
		}
	case k.Left, k.Rune == 'h':
		for i := 0; i < n; i++ {
			e.Buf.MoveLeft()
		}
	case k.Right, k.Rune == 'l':
		for i := 0; i < n; i++ {
			e.Buf.MoveRight()
		}
	case k.Home:
		e.Buf.MoveLineStart()
	case k.End:
		e.Buf.MoveLineEnd()
	case k.Backspace:
		for i := 0; i < n; i++ {
			e.Buf.DeleteRuneBackspace()
		}
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
		for i := 0; i < n; i++ {
			e.Buf.InsertRune('\n')
		}
		e.Mode = ModeInsert
	case k.Rune == 'O':
		e.Buf.MoveLineStart()
		for i := 0; i < n; i++ {
			e.Buf.InsertRune('\n')
			e.Buf.MoveUp()
		}
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

// runAction runs the action once (convenience for runActionWithCount(action, 1)).
func (e *Editor) runAction(action string) {
	e.runActionWithCount(action, 1)
}

// runActionWithCount runs the action n times. Motions and repeatable edits use n; mode switches and quit run once.
func (e *Editor) runActionWithCount(action string, n int) {
	if n <= 0 {
		n = 1
	}
	repeatable := map[string]bool{
		"move_left": true, "move_right": true, "move_up": true, "move_down": true,
		"move_line_start": true, "move_line_end": true,
		"next_word": true, "prev_word": true,
		"split_line": true, "delete_backspace": true, "delete_line": true,
	}
	switch action {
	case "open_line_below":
		e.Buf.MoveLineEnd()
		for i := 0; i < n; i++ {
			e.Buf.InsertRune('\n')
		}
		e.Mode = ModeInsert
		return
	case "open_line_above":
		e.Buf.MoveLineStart()
		for i := 0; i < n; i++ {
			e.Buf.InsertRune('\n')
			e.Buf.MoveUp()
		}
		e.Mode = ModeInsert
		return
	}
	if !repeatable[action] {
		n = 1
	}
	for i := 0; i < n; i++ {
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
		// Mode switches and quit only run once
		if e.Mode != ModeNormal || e.Quit {
			break
		}
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

	// Single number: jump to that line (1-based). E.g. ":23" -> line 23. ":0" -> first line.
	if len(parts) == 1 {
		if lineNum, err := strconv.Atoi(parts[0]); err == nil && lineNum >= 0 {
			row := lineNum - 1
			if row < 0 {
				row = 0
			}
			if row >= len(e.Buf.Lines) {
				row = len(e.Buf.Lines) - 1
			}
			e.Buf.Row = row
			e.Buf.Col = 0
			e.Buf.ClampCursor()
			return
		}
	}

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
			e.ShowPopup(e.Msg, PopupError)
			return
		}
		e.Msg = T(lang, "msg_saved")
		e.ShowPopup(e.Msg, PopupInfo)
	case "wq":
		if err := e.Buf.Save(); err != nil {
			e.Msg = fmt.Sprintf(T(lang, "msg_error"), err.Error())
			e.ShowPopup(e.Msg, PopupError)
			return
		}
		e.Msg = T(lang, "msg_saved")
		e.ShowPopup(e.Msg, PopupInfo)
		e.Quit = true
	default:
		e.Msg = fmt.Sprintf(T(lang, "msg_unknown_cmd"), parts[0])
		e.ShowPopup(e.Msg, PopupError)
	}
}
