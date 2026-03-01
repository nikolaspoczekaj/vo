package core

// Keybind defines a key binding: mode, key sequence, action.
type Keybind struct {
	Mode   string
	Keys   string
	Action string
}

// KeybindConfig holds keybinds; supports lookup and prefix check for chord keys (e.g. dd).
type KeybindConfig struct {
	Bindings []Keybind

	// mode -> keys -> action
	lookup map[string]map[string]string
	// mode -> set of keys that are prefix of a longer binding (e.g. "d" for "dd")
	prefix map[string]map[string]bool
}

// IsPrefix returns true if keys is a prefix of a longer binding (e.g. "d" for "dd").
func (c *KeybindConfig) IsPrefix(mode, keys string) bool {
	if c == nil || c.prefix == nil {
		return false
	}
	return c.prefix[mode][keys]
}

// NewKeybindConfig builds the lookup maps from a list of keybinds.
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

// Action returns the action for (mode, keys) or "" if not found.
func (c *KeybindConfig) Action(mode, keys string) string {
	if c == nil || c.lookup == nil {
		return ""
	}
	return c.lookup[mode][keys]
}
