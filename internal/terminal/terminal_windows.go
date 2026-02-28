//go:build windows

package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

const (
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING aktiviert ANSI-Escapesequenzen in der Konsole (Windows 10+).
	enableVirtualTerminalProcessing = 0x0004
)

type windowsTerm struct {
	stdin         *os.File
	stdout        *os.File
	state         *term.State
	reader        *bufio.Reader
	outModeSaved  uint32
	outVTEnabled bool
}

// New erstellt die Windows-Implementierung des Terminals.
// Aktiviert Virtual Terminal Processing für stdout, damit Clear/Cursor-Sequenzen funktionieren.
func New() (Terminal, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("stdin is not a terminal")
	}
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	t := &windowsTerm{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		state:  state,
		reader: bufio.NewReader(os.Stdin),
	}
	// ANSI für stdout aktivieren (Bildschirm löschen, Cursor setzen)
	if err := t.enableStdoutVT(); err != nil {
		_ = term.Restore(int(t.stdin.Fd()), state)
		return nil, fmt.Errorf("terminal: VT für Ausgabe aktivieren: %w", err)
	}
	return t, nil
}

// enableStdoutVT aktiviert ENABLE_VIRTUAL_TERMINAL_PROCESSING auf der Konsole.
// Wir nutzen GetStdHandle(STD_OUTPUT_HANDLE), damit die echte Konsole gemeint ist.
func (t *windowsTerm) enableStdoutVT() error {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return err
	}
	if h == windows.InvalidHandle {
		return fmt.Errorf("ungültiges Stdout-Handle")
	}
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return err
	}
	t.outModeSaved = mode
	mode |= enableVirtualTerminalProcessing
	if err := windows.SetConsoleMode(h, mode); err != nil {
		return err
	}
	t.outVTEnabled = true
	return nil
}

func (t *windowsTerm) Init() error {
	// Wechsel in den alternativen Bildschirmpuffer (unterstützt von Windows Terminal / VT)
	_, _ = t.stdout.WriteString("\x1b[?1049h")
	return nil
}

func (t *windowsTerm) Close() error {
	// Zurück zum Hauptbildschirmpuffer
	_, _ = t.stdout.WriteString("\x1b[?1049l")
	if t.outVTEnabled {
		h, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
		if h != windows.InvalidHandle {
			_ = windows.SetConsoleMode(h, t.outModeSaved)
		}
		t.outVTEnabled = false
	}
	if t.state != nil {
		return term.Restore(int(t.stdin.Fd()), t.state)
	}
	return nil
}

func (t *windowsTerm) ReadKey() (Key, error) {
	return parseKeyFromReader(t.reader)
}

func (t *windowsTerm) ReadKeyWithTimeout(timeoutMs int) (Key, error) {
	if timeoutMs <= 0 {
		return t.ReadKey()
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	t.stdin.SetReadDeadline(deadline)
	b, err := t.reader.ReadByte()
	t.stdin.SetReadDeadline(time.Time{})
	if err != nil {
		if isTimeout(err) {
			return Key{}, ErrTimeout
		}
		return Key{}, err
	}
	if err := t.reader.UnreadByte(); err != nil {
		return Key{}, err
	}
	return parseKeyWithFirstByte(b, t.reader)
}

func (t *windowsTerm) Size() (rows, cols int, err error) {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || h == windows.InvalidHandle {
		return 24, 80, nil
	}
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(h, &info); err != nil {
		return 24, 80, nil
	}
	cols = int(info.Window.Right - info.Window.Left + 1)
	rows = int(info.Window.Bottom - info.Window.Top + 1)
	if rows < 2 || cols < 1 {
		return 24, 80, nil
	}
	return rows, cols, nil
}

func (t *windowsTerm) Write(s string) (int, error) {
	return t.stdout.WriteString(s)
}

func (t *windowsTerm) WriteBytes(p []byte) (int, error) {
	return t.stdout.Write(p)
}

func (t *windowsTerm) MoveCursor(row, col int) error {
	_, err := t.stdout.WriteString(fmt.Sprintf("\x1b[%d;%dH", row, col))
	return err
}

func (t *windowsTerm) ClearScreen() error {
	_, err := t.stdout.WriteString("\x1b[2J\x1b[H")
	return err
}

func (t *windowsTerm) HideCursor() error {
	_, err := t.stdout.WriteString("\x1b[?25l")
	return err
}

func (t *windowsTerm) ShowCursor() error {
	_, err := t.stdout.WriteString("\x1b[?25h")
	return err
}

func (t *windowsTerm) Flush() error {
	return t.stdout.Sync()
}

func (t *windowsTerm) Stdin() io.Reader  { return t.stdin }
func (t *windowsTerm) Stdout() io.Writer { return t.stdout }

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	t, ok := err.(timeout)
	return ok && t.Timeout()
}
