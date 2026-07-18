package main

import (
	"fmt"
	"os"

	"github.com/crenshaw-dev/argocd-config/cmd/argocd-config/commands"
)

func main() {
	root := commands.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(commands.ExitCode(err))
	}
}
