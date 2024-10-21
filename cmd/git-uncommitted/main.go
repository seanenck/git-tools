// Package main handles various git uncommitted states for output
package main

import (
	"errors"
	"os"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/uncommitted"
)

func main() {
	cli.Fatal(gitUncommitted())
}

func gitUncommitted() error {
	args := os.Args
	mode := ""
	switch len(args) {
	case 1:
		break
	case 2:
		mode = args[1]
	default:
		return errors.New("unknown command")
	}

	return uncommitted.Current(uncommitted.Settings{Mode: mode, Writer: os.Stdout})
}
