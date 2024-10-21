// Package main handles a git motd for repository status+dotfiles
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/dotfiles"
	"github.com/seanenck/git-tools/internal/uncommitted"
)

type cmd struct {
	written bool
	name    string
	fxn     func(io.Writer) error
}

func (c *cmd) Write(data []byte) (int, error) {
	if !c.written {
		fmt.Printf("[%s]\n", c.name)
		fmt.Println("===")
		c.written = true
	}
	fmt.Println(string(data))
	return len(data), nil
}

func dotfileDiff(w io.Writer) error {
	settings := dotfiles.Settings{}
	settings.Mode = dotfiles.DiffMode
	settings.Writer = w
	return dotfiles.Do(settings)
}

func uncommit(w io.Writer) error {
	settings := uncommitted.Settings{}
	settings.Writer = w
	return uncommitted.Current(settings)
}

func main() {
	cli.Fatal(run())
}

func run() error {
	if len(os.Args) != 1 {
		return errors.New("no arguments allowed")
	}
	for _, c := range []*cmd{
		{name: "dotfiles", fxn: dotfileDiff},
		{name: "uncommitted", fxn: uncommit},
	} {
		if err := c.fxn(c); err != nil {
			return err
		}
	}
	return nil
}
