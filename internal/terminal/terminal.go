// Package terminal definiert die plattformunabhängige Schnittstelle für
// Terminal-Ein- und -Ausgabe. Die konkrete Implementierung wird per
// Build-Tag (windows / !windows) gewählt.
package terminal

import (
	"io"
	"errors"
)

// ErrTimeout wird von ReadKeyWithTimeout zurückgegeben, wenn innerhalb der Zeitspanne keine Taste kam.
var ErrTimeout = errors.New("key read timeout")

// Key repräsentiert eine gedrückte Taste (Sonderzeichen oder Rune).
type Key struct {
	Rune rune
	Ctrl bool
	Alt  bool
	// Special keys (0 wenn keine)
	Up, Down, Left, Right bool
	Enter, Backspace, Esc  bool
	Home, End              bool
}

// IsRune true, wenn eine normale Zeichentaste (kein Sonderzeichen).
func (k Key) IsRune() bool {
	return !k.Up && !k.Down && !k.Left && !k.Right &&
		!k.Enter && !k.Backspace && !k.Esc && !k.Home && !k.End
}

// Terminal ist die Schnittstelle für alle OS-spezifischen Terminal-Operationen.
// Der Core-Editor verwendet nur diese Schnittstelle.
type Terminal interface {
	// Init bereitet das Terminal für den Editor vor (z. B. Raw-Mode).
	Init() error
	// Close stellt den ursprünglichen Zustand wieder her.
	Close() error
	// ReadKey liest eine einzelne Tasten-Eingabe (blockierend).
	ReadKey() (Key, error)
	// ReadKeyWithTimeout wie ReadKey, gibt aber nach timeout ErrTimeout zurück wenn keine Taste kam.
	ReadKeyWithTimeout(timeoutMs int) (Key, error)
	// Size liefert Zeilen- und Spaltenanzahl des Terminals.
	Size() (rows, cols int, err error)
	// Write schreibt Text an die aktuelle Cursor-Position.
	Write(s string) (int, error)
	// WriteBytes wie Write, aber mit []byte (für effiziente Nutzung).
	WriteBytes(p []byte) (int, error)
	// MoveCursor setzt den Cursor auf (row, col); 1-basiert.
	MoveCursor(row, col int) error
	// ClearScreen löscht den sichtbaren Bereich und setzt Cursor auf (1,1).
	ClearScreen() error
	// HideCursor / ShowCursor für Cursor ein-/ausblenden.
	HideCursor() error
	ShowCursor() error
	// Flush schreibt alle gepufferten Ausgaben auf die Konsole (wichtig unter Windows).
	Flush() error
	// Stdin/Stdout für Fallbacks oder Tests (können nil sein).
	Stdin() io.Reader
	Stdout() io.Writer
}
