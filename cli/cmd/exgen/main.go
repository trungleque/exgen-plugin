// Command exgen installs a Claude Code plugin's skills, agents and commands
// into other coding tools.
package main

import (
	"fmt"
	"os"

	"github.com/trungleque/exgen-plugin/cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
