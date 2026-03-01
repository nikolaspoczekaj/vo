package terminal

import (
	"io"
)

// parseKeyFromReader reads from r until a complete key is recognized.
func parseKeyFromReader(r io.Reader) (Key, error) {
	var buf [1]byte
	n, err := r.Read(buf[:])
	if n < 1 || err != nil {
		return Key{}, err
	}
	return parseKeyWithFirstByte(buf[0], r)
}

// parseKeyWithFirstByte interprets the key whose first byte has already been read.
func parseKeyWithFirstByte(b byte, r io.Reader) (Key, error) {
	switch b {
	case KeyRuneEsc:
		return parseEscapeSequence(r)
	case KeyRuneEnter, '\n':
		return Key{Enter: true}, nil
	case KeyRuneBackspace:
		return Key{Backspace: true}, nil
	case KeyRuneTab:
		return Key{Rune: KeyRuneTab}, nil
	default:
		if b >= 32 && b < 127 {
			return Key{Rune: rune(b)}, nil
		}
		if b < 32 {
			return Key{Rune: rune(b + 64), Ctrl: true}, nil
		}
		return Key{Rune: rune(b)}, nil
	}
}

func parseEscapeSequence(r io.Reader) (Key, error) {
	// Read one more char; if '[' then CSI (e.g. arrows)
	if br, ok := r.(io.ByteReader); ok {
		c, err := br.ReadByte()
		if err != nil {
			return Key{Esc: true}, nil
		}
		if c != '[' {
			return Key{Esc: true}, nil
		}
		c, err = br.ReadByte()
		if err != nil {
			return Key{Esc: true}, nil
		}
		switch c {
		case 'A':
			return Key{Up: true}, nil
		case 'B':
			return Key{Down: true}, nil
		case 'C':
			return Key{Right: true}, nil
		case 'D':
			return Key{Left: true}, nil
		case 'H':
			return Key{Home: true}, nil
		case 'F':
			return Key{End: true}, nil
		}
		return Key{Esc: true}, nil
	}
	// Fallback: continue without ByteReader
	var buf [1]byte
	if _, err := r.Read(buf[:]); err != nil {
		return Key{Esc: true}, nil
	}
	if buf[0] != '[' {
		return Key{Esc: true}, nil
	}
	if _, err := r.Read(buf[:]); err != nil {
		return Key{Esc: true}, nil
	}
	switch buf[0] {
	case 'A':
		return Key{Up: true}, nil
	case 'B':
		return Key{Down: true}, nil
	case 'C':
		return Key{Right: true}, nil
	case 'D':
		return Key{Left: true}, nil
	case 'H':
		return Key{Home: true}, nil
	case 'F':
		return Key{End: true}, nil
	}
	return Key{Esc: true}, nil
}
