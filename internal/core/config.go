package core

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds the configuration loaded at startup (options and keybinds).
type Config struct {
	Options  map[string]string  // e.g. "timeout" -> "300"
	Keybinds *KeybindConfig     // built from keybind lines
}

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		Options: map[string]string{
			"timeout":             "300",
			"relative_linenumber": "false",
			"indent":              "4",
			"language":            "en",
		},
		Keybinds: defaultKeybinds(),
	}
}

// PendingTimeoutMs returns the timeout in milliseconds for chord keys (e.g. jj). Default 300 if unset or invalid.
func (c *Config) PendingTimeoutMs() int {
	if c == nil || c.Options == nil {
		return 300
	}
	s := c.Options["timeout"]
	if s == "" {
		return 300
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 300
	}
	return n
}

// RelativeLineNumber: true = relative line numbers (distance to current line), false = absolute.
func (c *Config) RelativeLineNumber() bool {
	if c == nil || c.Options == nil {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(c.Options["relative_linenumber"]))
	return s == "true" || s == "1" || s == "yes"
}

// IndentSize returns the number of spaces for Tab in Insert mode. Default 4.
func (c *Config) IndentSize() int {
	if c == nil || c.Options == nil {
		return 4
	}
	s := strings.TrimSpace(c.Options["indent"])
	if s == "" {
		return 4
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 32 {
		return 4
	}
	return n
}

// Language returns the UI language code (e.g. "en", "de"). Default "en" if not set or invalid.
func (c *Config) Language() string {
	if c == nil || c.Options == nil {
		return LangEN
	}
	s := strings.ToLower(strings.TrimSpace(c.Options["language"]))
	if s == "" || (s != LangEN && s != LangDE) {
		return LangEN
	}
	return s
}

// LoadConfig loads nim.conf line by line. Empty lines and # lines are ignored.
// Options: "timeout 300" or "timeout = 300". Keybinds: "keybind <mode> <keys> <action>".
// Config is loaded once and kept in memory.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := &Config{
		Options:  make(map[string]string),
		Keybinds: nil,
	}
	var bindings []Keybind

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Option: timeout 300  or  timeout = 300
		if strings.HasPrefix(line, "timeout") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "timeout"))
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				cfg.Options["timeout"] = rest
			}
			continue
		}
		if strings.HasPrefix(line, "relative_linenumber") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "relative_linenumber"))
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				cfg.Options["relative_linenumber"] = rest
			}
			continue
		}
		if strings.HasPrefix(line, "indent") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "indent"))
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				cfg.Options["indent"] = rest
			}
			continue
		}
		if strings.HasPrefix(line, "language") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "language"))
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				cfg.Options["language"] = rest
			}
			continue
		}
		// keybind <mode> <keys> <action>
		if strings.HasPrefix(line, "keybind ") {
			parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "keybind")))
			if len(parts) >= 3 {
				mode := parts[0]
				keys := parts[1]
				action := strings.Join(parts[2:], " ")
				bindings = append(bindings, Keybind{Mode: mode, Keys: keys, Action: action})
			}
			continue
		}
		// Other options: name value  or  name = value
		if idx := strings.Index(line, " "); idx > 0 {
			name := strings.TrimSpace(line[:idx])
			rest := strings.TrimSpace(line[idx+1:])
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)
			if name != "" && rest != "" {
				cfg.Options[name] = rest
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(bindings) > 0 {
		cfg.Keybinds = NewKeybindConfig(bindings)
	} else {
		cfg.Keybinds = defaultKeybinds()
	}

	// Defaults if not present in file
	if _, ok := cfg.Options["timeout"]; !ok {
		cfg.Options["timeout"] = "300"
	}
	if _, ok := cfg.Options["relative_linenumber"]; !ok {
		cfg.Options["relative_linenumber"] = "false"
	}
	if _, ok := cfg.Options["indent"]; !ok {
		cfg.Options["indent"] = "4"
	}
	if _, ok := cfg.Options["language"]; !ok {
		cfg.Options["language"] = "en"
	}

	return cfg, nil
}

// defaultKeybinds returns the built-in keybinds.
func defaultKeybinds() *KeybindConfig {
	return NewKeybindConfig([]Keybind{
		{Mode: "normal", Keys: "h", Action: "move_left"},
		{Mode: "normal", Keys: "j", Action: "move_down"},
		{Mode: "normal", Keys: "k", Action: "move_up"},
		{Mode: "normal", Keys: "l", Action: "move_right"},
		{Mode: "normal", Keys: "<Up>", Action: "move_up"},
		{Mode: "normal", Keys: "<Down>", Action: "move_down"},
		{Mode: "normal", Keys: "<Left>", Action: "move_left"},
		{Mode: "normal", Keys: "<Right>", Action: "move_right"},
		{Mode: "normal", Keys: "<Home>", Action: "move_line_start"},
		{Mode: "normal", Keys: "<End>", Action: "move_line_end"},
		{Mode: "normal", Keys: "<Enter>", Action: "split_line"},
		{Mode: "normal", Keys: "<Backspace>", Action: "delete_backspace"},
		{Mode: "normal", Keys: "i", Action: "insert"},
		{Mode: "normal", Keys: "a", Action: "insert_after"},
		{Mode: "normal", Keys: "A", Action: "insert_at_line_end"},
		{Mode: "normal", Keys: "o", Action: "open_line_below"},
		{Mode: "normal", Keys: "O", Action: "open_line_above"},
		{Mode: "normal", Keys: ":", Action: "command_mode"},
		{Mode: "normal", Keys: "<C-c>", Action: "quit"},
		{Mode: "normal", Keys: "dd", Action: "delete_line"},
		{Mode: "normal", Keys: "gg", Action: "buffer_start"},
		{Mode: "normal", Keys: "G", Action: "buffer_end"},
		{Mode: "normal", Keys: "0", Action: "move_line_start"},
		{Mode: "normal", Keys: "w", Action: "next_word"},
		{Mode: "normal", Keys: "b", Action: "prev_word"},
		{Mode: "insert", Keys: "jj", Action: "normal_mode"},
	})
}
