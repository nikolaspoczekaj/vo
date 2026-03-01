//go:build windows

package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

const (
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING enables ANSI escape sequences in the console (Windows 10+).
	enableVirtualTerminalProcessing = 0x0004
)

type windowsTerm struct {
	stdin         *os.File
	stdout        *os.File
	state         *term.State
	reader        *bufio.Reader
	outModeSaved  uint32
	outVTEnabled  bool
	mu            sync.Mutex
	pendingResult chan keyResult // after timeout, goroutine may send here; next read consumes it
}

// New returns the Windows implementation of the terminal. Enables Virtual Terminal Processing for stdout so clear/cursor sequences work.
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
	// Enable ANSI for stdout (clear screen, set cursor)
	if err := t.enableStdoutVT(); err != nil {
		_ = term.Restore(int(t.stdin.Fd()), state)
		return nil, fmt.Errorf("terminal: VT für Ausgabe aktivieren: %w", err)
	}
	return t, nil
}

// enableStdoutVT enables ENABLE_VIRTUAL_TERMINAL_PROCESSING on the console. Uses GetStdHandle(STD_OUTPUT_HANDLE) for the real console.
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
	// Switch to alternate screen buffer (supported by Windows Terminal / VT)
	_, _ = t.stdout.WriteString("\x1b[?1049h")
	return nil
}

func (t *windowsTerm) Close() error {
	// Back to main screen buffer
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
	t.mu.Lock()
	ch := t.pendingResult
	t.pendingResult = nil
	t.mu.Unlock()
	if ch != nil {
		res := <-ch
		return res.key, res.err
	}
	return parseKeyFromReader(t.reader)
}

func (t *windowsTerm) ReadKeyWithTimeout(timeoutMs int) (Key, error) {
	if timeoutMs <= 0 {
		return t.ReadKey()
	}
	t.mu.Lock()
	ch := t.pendingResult
	t.pendingResult = nil
	t.mu.Unlock()
	if ch != nil {
		res := <-ch
		return res.key, res.err
	}
	resultCh := make(chan keyResult, 1)
	go func() {
		k, err := parseKeyFromReader(t.reader)
		resultCh <- keyResult{k, err}
	}()
	select {
	case res := <-resultCh:
		return res.key, res.err
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		t.mu.Lock()
		t.pendingResult = resultCh
		t.mu.Unlock()
		return Key{}, ErrTimeout
	}
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
