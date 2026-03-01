// Nim – minimal Vim-like editor in Go.
// Usage: nim [filename]
package main

import (
	"fmt"
	"os"

	"nim/internal/core"
	"nim/internal/terminal"
)

func main() {
	buf := core.NewBuffer()
	if len(os.Args) >= 2 {
		path := os.Args[1]
		if err := buf.Load(path); err != nil {
			fmt.Fprintf(os.Stderr, "nim: %v\n", err)
			os.Exit(1)
		}
	}

	term, err := terminal.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nim: Terminal: %v\n", err)
		os.Exit(1)
	}

	config, _ := core.LoadConfig("nim.conf")
	ed := core.NewEditor(buf, term, config)
	if err := ed.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nim: %v\n", err)
		os.Exit(1)
	}
}
