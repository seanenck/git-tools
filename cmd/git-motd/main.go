// Package main handles a git motd for repository status+dotfiles
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/dotfiles"
	"github.com/seanenck/git-tools/internal/uncommitted"
)

type cmd struct {
	name    string
	written bool
	fxn     func(io.Writer) error
	buf     *bytes.Buffer
}

func (c *cmd) Write(data []byte) (int, error) {
	if !c.written {
		fmt.Fprintf(c.buf, "[%s]\n", c.name)
		fmt.Fprintln(c.buf, "===")
		c.written = true
	}
	fmt.Fprintln(c.buf, string(data))
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
	type result struct {
		name string
		err  error
		res  []byte
	}
	var all []chan result
	for _, c := range []*cmd{
		{name: "dotfiles", fxn: dotfileDiff},
		{name: "uncommitted", fxn: uncommit},
	} {
		r := make(chan result)
		c.buf = &bytes.Buffer{}
		go func(command *cmd, res chan result) {
			obj := result{name: command.name}
			if err := command.fxn(command); err == nil {
				obj.res = command.buf.Bytes()
			} else {
				obj.err = err
			}
			res <- obj
		}(c, r)
		all = append(all, r)
	}
	var collected []result
	var errs []error
	for _, a := range all {
		res := <-a
		if res.err != nil {
			errs = append(errs, res.err)
			continue
		}
		collected = append(collected, res)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	slices.SortFunc(collected, func(a, b result) int {
		return strings.Compare(a.name, b.name)
	})
	for _, item := range collected {
		fmt.Print(string(item.res))
	}
	return nil
}
