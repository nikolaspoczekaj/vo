// Package core: i18n provides translated user-facing strings. Language is set via config (e.g. language en).
package core

// Supported language codes: "en", "de".
const (
	LangEN = "en"
	LangDE = "de"
)

var messages = map[string]map[string]string{
	LangEN: {
		"no_name":        "[No Name]",
		"dirty_suffix":   " [+]",
		"status_fmt":    "%s%s | %s | %d:%d",
		"msg_unsaved":   "Unsaved changes ( :w to save, :q! to quit without saving )",
		"msg_saved":     "Saved",
		"msg_error":     "Error: %s",
		"msg_unknown_cmd": "Unknown command: %s",
		"mode_normal":   "NORMAL",
		"mode_insert":   "INSERT",
		"mode_command":  "COMMAND",
	},
	LangDE: {
		"no_name":        "[Unbenannt]",
		"dirty_suffix":   " [+]",
		"status_fmt":    "%s%s | %s | %d:%d",
		"msg_unsaved":   "Ungespeicherte Änderungen ( :w zum Speichern, :q! zum Verlassen ohne Speichern )",
		"msg_saved":     "Gespeichert",
		"msg_error":     "Fehler: %s",
		"msg_unknown_cmd": "Unbekannter Befehl: %s",
		"mode_normal":   "NORMAL",
		"mode_insert":   "EINFÜGEN",
		"mode_command":  "BEFEHL",
	},
}

// T returns the translation for key in the given language. Falls back to English if lang is unknown or key is missing.
func T(lang, key string) string {
	if messages[lang] != nil && messages[lang][key] != "" {
		return messages[lang][key]
	}
	if messages[LangEN][key] != "" {
		return messages[LangEN][key]
	}
	return key
}
