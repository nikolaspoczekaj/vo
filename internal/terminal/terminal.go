// Package terminal defines the platform-independent interface for terminal I/O.
// The concrete implementation is selected by build tag (windows / !windows).
package terminal

import (
	"io"
	"errors"
)

// ErrTimeout is returned by ReadKeyWithTimeout when no key was read within the timeout.
var ErrTimeout = errors.New("key read timeout")

// keyResult is used by platform implementations for buffering a key after timeout.
type keyResult struct {
	key Key
	err error
}

// Key represents a pressed key (special key or rune).
type Key struct {
	Rune rune
	Ctrl bool
	Alt  bool
	// Special keys (0 when none)
	Up, Down, Left, Right bool
	Enter, Backspace, Esc  bool
	Home, End              bool
}

// IsRune returns true for a normal character key (not a special key).
func (k Key) IsRune() bool {
	return !k.Up && !k.Down && !k.Left && !k.Right &&
		!k.Enter && !k.Backspace && !k.Esc && !k.Home && !k.End
}

// Terminal is the interface for all OS-specific terminal operations. The core editor uses only this interface.
type Terminal interface {
	// Init prepares the terminal for the editor (e.g. raw mode).
	Init() error
	// Close restores the original terminal state.
	Close() error
	// ReadKey reads a single key press (blocking).
	ReadKey() (Key, error)
	// ReadKeyWithTimeout like ReadKey, but returns ErrTimeout after timeout if no key was pressed.
	ReadKeyWithTimeout(timeoutMs int) (Key, error)
	// Size returns the terminal row and column count.
	Size() (rows, cols int, err error)
	// Write writes text at the current cursor position.
	Write(s string) (int, error)
	// WriteBytes like Write but with []byte (for efficiency).
	WriteBytes(p []byte) (int, error)
	// MoveCursor sets the cursor to (row, col); 1-based.
	MoveCursor(row, col int) error
	// ClearScreen clears the visible area and sets cursor to (1,1).
	ClearScreen() error
	// HideCursor / ShowCursor to hide or show the cursor.
	HideCursor() error
	ShowCursor() error
	// Flush flushes all buffered output to the console (important on Windows).
	Flush() error
	// Stdin/Stdout for fallbacks or tests (may be nil).
	Stdin() io.Reader
	Stdout() io.Writer
}
