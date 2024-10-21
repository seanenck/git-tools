// Package main handles current state for a git repo
package main

import (
	"flag"
	"os"
	"strings"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/state"
)

const branchDelimiter = ","

func main() {
	cli.Fatal(gitCurrentState())
}

func gitCurrentState() error {
	quick := flag.Bool("quick", false, "quickly exit on first issue")
	branches := flag.String("default-branches", strings.Join(state.DefaultBranches, branchDelimiter), "default branch names")
	flag.Parse()
	var useBranches []string
	branching := strings.TrimSpace(*branches)
	if branching != "" {
		useBranches = strings.Split(branching, branchDelimiter)
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return state.Current(state.Settings{Quick: *quick, Branches: useBranches, Writer: os.Stdout, Dir: wd})
}
