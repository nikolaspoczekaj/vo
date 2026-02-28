package terminal

// Häufige Sonderzeichen als Konstanten.
const (
	KeyRuneEnter     = '\r'
	KeyRuneBackspace = '\x7f'
	KeyRuneEsc       = '\x1b'
	KeyRuneTab       = '\t'
)

// ConfigString liefert die in Keybind-Configs verwendete Bezeichnung für diese Taste.
// z. B. "h", "<Up>", "<C-c>"
func (k Key) ConfigString() string {
	if k.Up {
		return "<Up>"
	}
	if k.Down {
		return "<Down>"
	}
	if k.Left {
		return "<Left>"
	}
	if k.Right {
		return "<Right>"
	}
	if k.Enter {
		return "<Enter>"
	}
	if k.Backspace {
		return "<Backspace>"
	}
	if k.Esc {
		return "<Esc>"
	}
	if k.Home {
		return "<Home>"
	}
	if k.End {
		return "<End>"
	}
	if k.Ctrl && k.Rune != 0 {
		return "<C-" + string(toLower(k.Rune)) + ">"
	}
	if k.Rune != 0 {
		return string(k.Rune)
	}
	return ""
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
