// Package main handles current state for a git repo
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/seanenck/git-tools/internal/cli"
)

type (
	gitStatus struct {
		cmd string
		ok  bool
		err error
		dir gitPath
	}
	gitPath string
)

func main() {
	cli.Fatal(gitCurrentState())
}

func (r gitStatus) write() {
	fmt.Printf("-> %s (%s)\n", r.dir, r.cmd)
}

func gitCommand(sub string, p gitPath, filter []string, args ...string) gitStatus {
	resulting := gitStatus{cmd: sub, dir: p}
	arguments := []string{sub}
	arguments = append(arguments, args...)
	cmd := exec.Command("git", arguments...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err == nil {
		trimmed := strings.TrimSpace(string(out))
		if len(filter) == 0 {
			resulting.ok = trimmed == ""
		} else {
			resulting.ok = slices.Contains(filter, trimmed)
		}
	} else {
		resulting.err = err
	}
	return resulting
}

func gitCurrentState() error {
	quick := flag.Bool("quick", false, "quickly exit on first issue")
	branches := flag.String("default-branches", "main,master", "default branch names")
	flag.Parse()
	var useBranches []string
	branching := strings.TrimSpace(*branches)
	if branching != "" {
		useBranches = strings.Split(branching, ",")
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	color := func(text string, mode int) {
		fmt.Printf("\x1b[%dm(%s)\x1b[0m", mode, text)
	}
	dirty := func() {
		color("dirty", 31)
	}
	directory := gitPath(dir)
	if directory == "" {
		return errors.New("directory must be set")
	}
	isQuick := *quick
	r := gitCommand("update-index", directory, []string{}, "-q", "--refresh")
	if r.err != nil {
		return r.err
	}
	if !r.ok {
		if isQuick {
			dirty()
			return nil
		}
		r.write()
	}
	isBranch := "branch"
	gitCommandAsync := func(res chan gitStatus, sub string, p gitPath, args ...string) {
		filter := []string{}
		if sub == isBranch {
			filter = useBranches
		}
		res <- gitCommand(sub, p, filter, args...)
	}
	cmds := map[string][]string{
		"diff-index": {"--name-only", "HEAD", "--"},
		"log":        {"--branches", "--not", "--remotes", "-n", "1"},
		"ls-files":   {"--others", "--exclude-standard", "--directory", "--no-empty-directory"},
	}
	if len(useBranches) > 0 {
		cmds[isBranch] = []string{"--show-current"}
	}
	var results []chan gitStatus
	for sub, cmd := range cmds {
		r := make(chan gitStatus)
		go gitCommandAsync(r, sub, directory, cmd...)
		results = append(results, r)
	}

	done := false
	for _, r := range results {
		read := <-r
		if read.err != nil {
			continue
		}
		if !read.ok {
			if isQuick && !done {
				dirty()
				done = true
			}
			if !isQuick {
				read.write()
			}
		}
	}
	if done {
		return nil
	}

	if isQuick {
		color("clean", 32)
	}
	return nil
}
