// Vo – minimal Vim-like editor in Go.
// Usage: vo [filename]
package main

import (
	"fmt"
	"os"

	"github.com/nikolaspoczekaj/vo/internal/core"
	"github.com/nikolaspoczekaj/vo/internal/logging"
	"github.com/nikolaspoczekaj/vo/internal/terminal"
)

func main() {
	buf := core.NewBuffer()
	if len(os.Args) >= 2 {
		path := os.Args[1]
		if err := buf.Load(path); err != nil {
			fmt.Fprintf(os.Stderr, "vo: %v\n", err)
			os.Exit(1)
		}
	}

	term, err := terminal.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vo: Terminal: %v\n", err)
		os.Exit(1)
	}

	configPath, created, err := core.EnsureConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vo: Config: %v\n", err)
		os.Exit(1)
	}
	config, _ := core.LoadConfig(configPath)
	ed := core.NewEditor(buf, term, config)

	// Wire global logging popups to the editor's popup system.
	logging.SetPopupFunc(func(text string, level logging.Level) {
		kind := core.PopupInfo
		if level == logging.LevelWarn || level == logging.LevelError {
			kind = core.PopupError
		}
		ed.ShowPopup(text, kind)
	})

	// If the config file was just created, log and show an informational popup on startup.
	if created {
		lang := core.LangEN
		if config != nil {
			lang = config.Language()
		}
		msg := fmt.Sprintf(core.T(lang, "msg_config_created"), configPath)
		logging.Info(msg, true)
	}
	if err := ed.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vo: %v\n", err)
		os.Exit(1)
	}
}
