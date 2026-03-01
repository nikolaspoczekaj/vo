// Package core contains the platform-independent editor logic (buffer, modes, commands).
package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Buffer holds the text of a file and the cursor position.
type Buffer struct {
	Lines []string
	Path  string
	Row   int // 0-based
	Col   int // 0-based, not past line end
	Dirty bool
}

// NewBuffer creates an empty buffer.
func NewBuffer() *Buffer {
	return &Buffer{
		Lines: []string{""},
		Row:   0,
		Col:   0,
	}
}

// Load loads a file into the buffer.
func (b *Buffer) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			b.Path = path
			b.Lines = []string{""}
			b.Row, b.Col = 0, 0
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	b.Lines = nil
	for sc.Scan() {
		b.Lines = append(b.Lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(b.Lines) == 0 {
		b.Lines = []string{""}
	}
	b.Path = path
	b.Row, b.Col = 0, 0
	b.Dirty = false
	return nil
}

// Save writes the buffer to the file.
func (b *Buffer) Save() error {
	if b.Path == "" {
		return fmt.Errorf("no file path")
	}
	f, err := os.Create(b.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	for i, line := range b.Lines {
		if i > 0 {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
		if _, err := f.WriteString(line); err != nil {
			return err
		}
	}
	b.Dirty = false
	return nil
}

// CurrentLine returns the current line (never nil).
func (b *Buffer) CurrentLine() string {
	if b.Row < 0 || b.Row >= len(b.Lines) {
		return ""
	}
	return b.Lines[b.Row]
}

// LineCount returns the number of lines.
func (b *Buffer) LineCount() int {
	return len(b.Lines)
}

// ClampCursor keeps Row/Col within valid bounds.
func (b *Buffer) ClampCursor() {
	if b.Row < 0 {
		b.Row = 0
	}
	if b.Row >= len(b.Lines) {
		b.Row = len(b.Lines) - 1
	}
	lineLen := len(b.CurrentLine())
	if b.Col < 0 {
		b.Col = 0
	}
	if b.Col > lineLen {
		b.Col = lineLen
	}
}

// InsertRune inserts a rune at the cursor position (Insert mode).
func (b *Buffer) InsertRune(r rune) {
	b.ClampCursor()
	line := b.CurrentLine()
	left := line[:b.Col]
	right := line[b.Col:]
	if r == '\n' {
		b.Lines[b.Row] = left
		newLine := right
		b.Lines = append(b.Lines[:b.Row+1], append([]string{newLine}, b.Lines[b.Row+1:]...)...)
		b.Row++
		b.Col = 0
	} else {
		b.Lines[b.Row] = left + string(r) + right
		b.Col++
	}
	b.Dirty = true
}

// InsertSpaces inserts n spaces at the cursor (e.g. for Tab with indent).
func (b *Buffer) InsertSpaces(n int) {
	if n <= 0 {
		return
	}
	b.ClampCursor()
	line := b.CurrentLine()
	left := line[:b.Col]
	right := line[b.Col:]
	spaces := strings.Repeat(" ", n)
	b.Lines[b.Row] = left + spaces + right
	b.Col += n
	b.Dirty = true
}

// DeleteRuneBackspace deletes the character before the cursor (Backspace).
func (b *Buffer) DeleteRuneBackspace() {
	b.ClampCursor()
	if b.Col > 0 {
		line := b.CurrentLine()
		b.Lines[b.Row] = line[:b.Col-1] + line[b.Col:]
		b.Col--
		b.Dirty = true
	} else if b.Row > 0 {
		// Merge with previous line
		prev := b.Lines[b.Row-1]
		b.Lines = append(b.Lines[:b.Row-1], b.Lines[b.Row:]...)
		b.Row--
		b.Col = len(prev)
		b.Lines[b.Row] = prev + b.CurrentLine()
		b.Dirty = true
	}
}

// MoveLeft moves the cursor one character left.
func (b *Buffer) MoveLeft() {
	b.ClampCursor()
	if b.Col > 0 {
		b.Col--
	}
}

// MoveRight moves the cursor one character right.
func (b *Buffer) MoveRight() {
	b.ClampCursor()
	if b.Col < len(b.CurrentLine()) {
		b.Col++
	}
}

// MoveUp moves the cursor one line up.
func (b *Buffer) MoveUp() {
	if b.Row > 0 {
		b.Row--
		b.ClampCursor()
	}
}

// MoveDown moves the cursor one line down.
func (b *Buffer) MoveDown() {
	if b.Row < len(b.Lines)-1 {
		b.Row++
		b.ClampCursor()
	}
}

// MoveLineStart sets the cursor to the start of the line.
func (b *Buffer) MoveLineStart() {
	b.Col = 0
}

// MoveLineEnd sets the cursor to the end of the line.
func (b *Buffer) MoveLineEnd() {
	b.Col = len(b.CurrentLine())
}

// MoveBufferStart sets the cursor to the start of the buffer (first line, first column).
func (b *Buffer) MoveBufferStart() {
	b.Row = 0
	b.Col = 0
	b.ClampCursor()
}

// MoveBufferEnd sets the cursor to the end of the buffer (last line, line end).
func (b *Buffer) MoveBufferEnd() {
	if len(b.Lines) == 0 {
		return
	}
	b.Row = len(b.Lines) - 1
	b.MoveLineEnd()
}

// isWordByte is true for letters, digits, and underscore (word characters as in Vim).
func isWordByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// isBlank is true for space and tab.
func isBlank(c byte) bool {
	return c == ' ' || c == '\t'
}

// MoveToNextWord (w) moves the cursor to the start of the next word.
// As in Vim: a "word" is either a run of word characters (letters, digits, _) or a run of
// other non-blank characters (e.g. brackets, #-). So in "(foo)" you can step before "(", "foo", ")".
func (b *Buffer) MoveToNextWord() {
	b.ClampCursor()
	line := b.CurrentLine()
	col := b.Col
	startCol := col
	// Only skip when we're *inside* the same word run (previous char same type)
	if col < len(line) && col > 0 && !isBlank(line[col-1]) {
		prevIsWord := isWordByte(line[col-1])
		currIsWord := col < len(line) && isWordByte(line[col])
		if prevIsWord == currIsWord && (prevIsWord || !isBlank(line[col])) {
			if prevIsWord {
				for col < len(line) && isWordByte(line[col]) {
					col++
				}
			} else {
				for col < len(line) && !isBlank(line[col]) {
					col++
				}
			}
		}
	}
	// Skip spaces/tabs
	for col < len(line) && isBlank(line[col]) {
		col++
	}
	// Already at word start (or on blank)? Skip this word so we land on the next.
	if col == startCol && col < len(line) && !isBlank(line[col]) {
		if isWordByte(line[col]) {
			for col < len(line) && isWordByte(line[col]) {
				col++
			}
		} else {
			for col < len(line) && !isBlank(line[col]) {
				col++
			}
		}
		for col < len(line) && isBlank(line[col]) {
			col++
		}
	}
	if col < len(line) {
		b.Col = col
		return
	}
	// End of line: next line
	if b.Row >= len(b.Lines)-1 {
		b.Col = len(line)
		return
	}
	b.Row++
	b.Col = 0
	line = b.CurrentLine()
	for b.Col < len(line) && isBlank(line[b.Col]) {
		b.Col++
	}
}

// MoveToPrevWord (b) moves the cursor to the start of the current or previous word.
// As in Vim: word = word-char run or non-blank run (e.g. brackets).
func (b *Buffer) MoveToPrevWord() {
	b.ClampCursor()
	line := b.CurrentLine()
	col := b.Col
	// Inside a word/run: go to start of this run
	if col > 0 {
		if isWordByte(line[col-1]) {
			for col > 0 && isWordByte(line[col-1]) {
				col--
			}
			b.Col = col
			return
		}
		if !isBlank(line[col-1]) {
			for col > 0 && !isBlank(line[col-1]) {
				col--
			}
			b.Col = col
			return
		}
	}
	// Skip blanks backward
	for col > 0 && isBlank(line[col-1]) {
		col--
	}
	// Previous word/run
	if col > 0 {
		if isWordByte(line[col-1]) {
			for col > 0 && isWordByte(line[col-1]) {
				col--
			}
		} else {
			for col > 0 && !isBlank(line[col-1]) {
				col--
			}
		}
		b.Col = col
		return
	}
	// At line start: go up one line, last word
	if b.Row == 0 {
		b.Col = 0
		return
	}
	b.Row--
	line = b.CurrentLine()
	b.Col = len(line)
	for b.Col > 0 && isBlank(line[b.Col-1]) {
		b.Col--
	}
	if b.Col > 0 {
		if isWordByte(line[b.Col-1]) {
			for b.Col > 0 && isWordByte(line[b.Col-1]) {
				b.Col--
			}
		} else {
			for b.Col > 0 && !isBlank(line[b.Col-1]) {
				b.Col--
			}
		}
	}
}

// DeleteLine deletes the current line.
func (b *Buffer) DeleteLine() {
	if len(b.Lines) == 0 {
		return
	}
	b.Lines = append(b.Lines[:b.Row], b.Lines[b.Row+1:]...)
	if len(b.Lines) == 0 {
		b.Lines = []string{""}
	}
	b.Row--
	if b.Row < 0 {
		b.Row = 0
	}
	b.Col = 0
	b.ClampCursor()
	b.Dirty = true
}

// StatusLine returns a short status text (filename, mode, position). lang is the UI language code (e.g. "en", "de").
func (b *Buffer) StatusLine(lang, mode string) string {
	path := b.Path
	if path == "" {
		path = T(lang, "no_name")
	}
	dirty := ""
	if b.Dirty {
		dirty = T(lang, "dirty_suffix")
	}
	return fmt.Sprintf(T(lang, "status_fmt"), path, dirty, mode, b.Row+1, b.Col+1)
}

// VisibleLines returns the lines for display (from rowStart, up to n lines).
func (b *Buffer) VisibleLines(rowStart, n int) []string {
	if rowStart < 0 {
		rowStart = 0
	}
	var out []string
	for i := 0; i < n && rowStart+i < len(b.Lines); i++ {
		out = append(out, b.Lines[rowStart+i])
	}
	return out
}
