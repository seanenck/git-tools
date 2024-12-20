// Package main handles a git motd for repository status
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
	fmt.Fprint(c.buf, string(data))
	return len(data), nil
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
	enabled := strings.Split(cli.GitConfigValue("motd.enable"), " ")
	var all []chan result
	for _, c := range []*cmd{
		{name: "uncommitted", fxn: uncommit},
	} {
		if !slices.Contains(enabled, c.name) {
			continue
		}
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
	had := false
	for idx, item := range collected {
		if idx > 0 && had {
			fmt.Println()
		}
		had = len(item.res) > 0
		fmt.Print(string(item.res))
	}
	return nil
}
