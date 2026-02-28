package core

// Keybind definiert eine Tastenbindung: Modus, Tastenfolge, Aktion.
type Keybind struct {
	Mode   string
	Keys   string
	Action string
}

// KeybindConfig lädt und hält Keybinds; ermöglicht Lookup und Präfix-Prüfung für Mehrfach-Tasten (z. B. dd).
type KeybindConfig struct {
	Bindings []Keybind

	// mode -> keys -> action
	lookup map[string]map[string]string
	// mode -> set of keys that are prefix of a longer binding (e.g. "d" for "dd")
	prefix map[string]map[string]bool
}

// IsPrefix true, wenn keys ein Präfix einer längeren Bindung ist (z. B. "d" für "dd").
func (c *KeybindConfig) IsPrefix(mode, keys string) bool {
	if c == nil || c.prefix == nil {
		return false
	}
	return c.prefix[mode][keys]
}

// NewKeybindConfig baut aus einer Liste von Keybinds die Lookup-Maps.
func NewKeybindConfig(bindings []Keybind) *KeybindConfig {
	lookup := make(map[string]map[string]string)
	prefix := make(map[string]map[string]bool)
	for _, b := range bindings {
		if lookup[b.Mode] == nil {
			lookup[b.Mode] = make(map[string]string)
			prefix[b.Mode] = make(map[string]bool)
		}
		lookup[b.Mode][b.Keys] = b.Action
		if len(b.Keys) >= 2 {
			prefix[b.Mode][b.Keys[:1]] = true
		}
	}
	return &KeybindConfig{
		Bindings: bindings,
		lookup:   lookup,
		prefix:   prefix,
	}
}

// Action liefert die Aktion für (mode, keys) oder "" wenn nicht vorhanden.
func (c *KeybindConfig) Action(mode, keys string) string {
	if c == nil || c.lookup == nil {
		return ""
	}
	return c.lookup[mode][keys]
}
