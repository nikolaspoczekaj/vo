//go:build !windows

package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

type unixTerm struct {
	stdin         *os.File
	stdout        *os.File
	state         *term.State
	reader        *bufio.Reader
	mu            sync.Mutex
	pendingResult chan keyResult
}

// New returns the Unix implementation of the terminal (raw mode, ANSI).
func New() (Terminal, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("stdin is not a terminal")
	}
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	return &unixTerm{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		state:  state,
		reader: bufio.NewReader(os.Stdin),
	}, nil
}

func (t *unixTerm) Init() error {
	// Switch to alternate screen buffer (no flicker, scrollback preserved)
	_, _ = t.stdout.WriteString("\x1b[?1049h")
	return nil
}

func (t *unixTerm) Close() error {
	// Back to main screen buffer
	_, _ = t.stdout.WriteString("\x1b[?1049l")
	if t.state != nil {
		return term.Restore(int(t.stdin.Fd()), t.state)
	}
	return nil
}

func (t *unixTerm) ReadKey() (Key, error) {
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

func (t *unixTerm) ReadKeyWithTimeout(timeoutMs int) (Key, error) {
	if timeoutMs <= 0 {
		return t.ReadKey()
	}
	t.mu.Lock()
	ch := t.pendingResult
	t.mu.Unlock()
	if ch != nil {
		select {
		case res := <-ch:
			t.mu.Lock()
			t.pendingResult = nil
			t.mu.Unlock()
			return res.key, res.err
		case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
			return Key{}, ErrTimeout
		}
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

func (t *unixTerm) Size() (rows, cols int, err error) {
	return term.GetSize(int(t.stdin.Fd()))
}

func (t *unixTerm) Write(s string) (int, error) {
	return t.stdout.WriteString(s)
}

func (t *unixTerm) WriteBytes(p []byte) (int, error) {
	return t.stdout.Write(p)
}

func (t *unixTerm) MoveCursor(row, col int) error {
	_, err := t.stdout.WriteString(fmt.Sprintf("\x1b[%d;%dH", row, col))
	return err
}

func (t *unixTerm) ClearScreen() error {
	_, err := t.stdout.WriteString("\x1b[2J\x1b[H")
	return err
}

func (t *unixTerm) HideCursor() error {
	_, err := t.stdout.WriteString("\x1b[?25l")
	return err
}

func (t *unixTerm) ShowCursor() error {
	_, err := t.stdout.WriteString("\x1b[?25h")
	return err
}

func (t *unixTerm) Flush() error {
	return t.stdout.Sync()
}

func (t *unixTerm) Stdin() io.Reader  { return t.stdin }
func (t *unixTerm) Stdout() io.Writer { return t.stdout }
