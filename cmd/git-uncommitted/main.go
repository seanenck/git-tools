// Package main handles various git uncommitted states for output
package main

import (
	"flag"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/uncommitted"
)

func main() {
	cli.Fatal(gitUncommitted())
}

func gitUncommitted() error {
	mode := flag.String("mode", "", "operating mode")
	flag.Parse()
	return uncommitted.Current(uncommitted.Settings{Mode: *mode})
}
