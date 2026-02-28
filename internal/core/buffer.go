// Package core enthält die plattformunabhängige Editor-Logik (Buffer, Modi, Befehle).
package core

import (
	"bufio"
	"fmt"
	"os"
)

// Buffer hält den Text einer Datei und die Cursor-Position.
type Buffer struct {
	Lines []string
	Path  string
	Row   int // 0-basiert
	Col   int // 0-basiert, nicht über Zeilenende
	Dirty bool
}

// NewBuffer erstellt einen leeren Buffer.
func NewBuffer() *Buffer {
	return &Buffer{
		Lines: []string{""},
		Row:   0,
		Col:   0,
	}
}

// Load lädt eine Datei in den Buffer.
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

// Save schreibt den Buffer in die Datei.
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

// CurrentLine gibt die aktuelle Zeile zurück (niemals nil).
func (b *Buffer) CurrentLine() string {
	if b.Row < 0 || b.Row >= len(b.Lines) {
		return ""
	}
	return b.Lines[b.Row]
}

// LineCount gibt die Anzahl der Zeilen zurück.
func (b *Buffer) LineCount() int {
	return len(b.Lines)
}

// ClampCursor hält Row/Col in gültigen Grenzen.
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

// InsertRune fügt ein Zeichen an der Cursor-Position ein (Insert-Modus).
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

// DeleteRuneBackspace löscht das Zeichen vor dem Cursor (Backspace).
func (b *Buffer) DeleteRuneBackspace() {
	b.ClampCursor()
	if b.Col > 0 {
		line := b.CurrentLine()
		b.Lines[b.Row] = line[:b.Col-1] + line[b.Col:]
		b.Col--
		b.Dirty = true
	} else if b.Row > 0 {
		// Zeile mit vorheriger zusammenführen
		prev := b.Lines[b.Row-1]
		b.Lines = append(b.Lines[:b.Row-1], b.Lines[b.Row:]...)
		b.Row--
		b.Col = len(prev)
		b.Lines[b.Row] = prev + b.CurrentLine()
		b.Dirty = true
	}
}

// MoveLeft bewegt den Cursor ein Zeichen nach links.
func (b *Buffer) MoveLeft() {
	b.ClampCursor()
	if b.Col > 0 {
		b.Col--
	}
}

// MoveRight bewegt den Cursor ein Zeichen nach rechts.
func (b *Buffer) MoveRight() {
	b.ClampCursor()
	if b.Col < len(b.CurrentLine()) {
		b.Col++
	}
}

// MoveUp bewegt den Cursor eine Zeile nach oben.
func (b *Buffer) MoveUp() {
	if b.Row > 0 {
		b.Row--
		b.ClampCursor()
	}
}

// MoveDown bewegt den Cursor eine Zeile nach unten.
func (b *Buffer) MoveDown() {
	if b.Row < len(b.Lines)-1 {
		b.Row++
		b.ClampCursor()
	}
}

// MoveLineStart setzt den Cursor auf den Zeilenanfang.
func (b *Buffer) MoveLineStart() {
	b.Col = 0
}

// MoveLineEnd setzt den Cursor auf das Zeilenende.
func (b *Buffer) MoveLineEnd() {
	b.Col = len(b.CurrentLine())
}

// MoveBufferStart setzt den Cursor an den Anfang des Buffers (erste Zeile, erste Spalte).
func (b *Buffer) MoveBufferStart() {
	b.Row = 0
	b.Col = 0
	b.ClampCursor()
}

// MoveBufferEnd setzt den Cursor ans Ende des Buffers (letzte Zeile, Zeilenende).
func (b *Buffer) MoveBufferEnd() {
	if len(b.Lines) == 0 {
		return
	}
	b.Row = len(b.Lines) - 1
	b.MoveLineEnd()
}

// isWordByte gilt für Buchstaben, Ziffern und Unterstrich (Wort im vim-Sinn).
func isWordByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// MoveToNextWord (w) setzt den Cursor auf den Start des nächsten Wortes.
func (b *Buffer) MoveToNextWord() {
	b.ClampCursor()
	line := b.CurrentLine()
	col := b.Col
	// Aus aktuellem Wort heraus (falls mitten drin)
	for col < len(line) && isWordByte(line[col]) {
		col++
	}
	// Über Nicht-Wort-Zeichen (Leerzeichen, Satzzeichen) zum nächsten Wort
	for col < len(line) && !isWordByte(line[col]) {
		col++
	}
	if col < len(line) {
		b.Col = col
		return
	}
	// Zeilenende: nächste Zeile
	if b.Row >= len(b.Lines)-1 {
		b.Col = len(line)
		return
	}
	b.Row++
	b.Col = 0
	line = b.CurrentLine()
	for b.Col < len(line) && !isWordByte(line[b.Col]) {
		b.Col++
	}
}

// MoveToPrevWord (b) setzt den Cursor auf den Anfang des aktuellen oder des vorherigen Wortes.
func (b *Buffer) MoveToPrevWord() {
	b.ClampCursor()
	line := b.CurrentLine()
	col := b.Col
	// Mitten im Wort oder direkt dahinter: zum Anfang dieses Wortes
	if col > 0 && isWordByte(line[col-1]) {
		for col > 0 && isWordByte(line[col-1]) {
			col--
		}
		b.Col = col
		return
	}
	// Über Nicht-Wort-Zeichen zurück
	for col > 0 && !isWordByte(line[col-1]) {
		col--
	}
	// Über Wort zurück → Start des vorherigen Wortes
	for col > 0 && isWordByte(line[col-1]) {
		col--
	}
	if col > 0 {
		b.Col = col
		return
	}
	// Am Zeilenanfang: eine Zeile nach oben, letztes Wort
	if b.Row == 0 {
		b.Col = 0
		return
	}
	b.Row--
	line = b.CurrentLine()
	b.Col = len(line)
	for b.Col > 0 && !isWordByte(line[b.Col-1]) {
		b.Col--
	}
	for b.Col > 0 && isWordByte(line[b.Col-1]) {
		b.Col--
	}
}

// DeleteLine löscht die aktuelle Zeile.
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

// StatusLine liefert einen kurzen Status-Text (Dateiname, Modus, Position).
func (b *Buffer) StatusLine(mode string) string {
	path := b.Path
	if path == "" {
		path = "[No Name]"
	}
	dirty := ""
	if b.Dirty {
		dirty = " [+]"
	}
	return fmt.Sprintf("%s%s | %s | %d:%d", path, dirty, mode, b.Row+1, b.Col+1)
}

// VisibleLines liefert die Zeilen für die Anzeige (von rowStart, max n Zeilen).
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
