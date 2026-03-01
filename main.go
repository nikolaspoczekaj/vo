// Vo – minimal Vim-like editor in Go.
// Usage: vo [filename]
package main

import (
	"fmt"
	"os"

	"github.com/nikolaspoczekaj/vo/internal/core"
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

	config, _ := core.LoadConfig("vo.conf")
	ed := core.NewEditor(buf, term, config)
	if err := ed.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vo: %v\n", err)
		os.Exit(1)
	}
}
