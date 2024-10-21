// Package state handles repository state
package state

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// DefaultBranches are the default branches to consider as 'default' (not specifically checked out)
var DefaultBranches = []string{"main", "master"}

type (
	gitStatus struct {
		cmd string
		ok  bool
		err error
		dir gitPath
	}
	gitPath string
	// Settings are the state settings to operate on
	Settings struct {
		Quick    bool
		Branches []string
		Writer   io.Writer
		Dir      string
	}
)

func (r gitStatus) write(w io.Writer) {
	fmt.Fprintf(w, "-> %s (%s)\n", r.dir, r.cmd)
}

func gitCommand(sub string, p gitPath, filter []string, args ...string) gitStatus {
	resulting := gitStatus{cmd: sub, dir: p}
	arguments := []string{sub}
	arguments = append(arguments, args...)
	cmd := exec.Command("git", arguments...)
	cmd.Stderr = os.Stderr
	cmd.Dir = string(resulting.dir)
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

// Current will write state for the inputs
func Current(settings Settings) error {
	if settings.Writer == nil {
		return errors.New("writer is unset")
	}
	if settings.Dir == "" {
		return errors.New("dir not set")
	}

	color := func(text string, mode int) {
		fmt.Fprintf(settings.Writer, "\x1b[%dm(%s)\x1b[0m", mode, text)
	}
	dirty := func() {
		color("dirty", 31)
	}
	directory := gitPath(settings.Dir)
	if directory == "" {
		return errors.New("directory must be set")
	}
	r := gitCommand("update-index", directory, []string{}, "-q", "--refresh")
	if r.err != nil {
		return r.err
	}
	if !r.ok {
		if settings.Quick {
			dirty()
			return nil
		}
		r.write(settings.Writer)
	}
	isBranch := "branch"
	gitCommandAsync := func(res chan gitStatus, sub string, p gitPath, args ...string) {
		filter := []string{}
		if sub == isBranch {
			filter = settings.Branches
		}
		res <- gitCommand(sub, p, filter, args...)
	}
	cmds := map[string][]string{
		"diff-index": {"--name-only", "HEAD", "--"},
		"log":        {"--branches", "--not", "--remotes", "-n", "1"},
		"ls-files":   {"--others", "--exclude-standard", "--directory", "--no-empty-directory"},
	}
	if len(settings.Branches) > 0 {
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
			if settings.Quick && !done {
				dirty()
				done = true
			}
			if !settings.Quick {
				read.write(settings.Writer)
			}
		}
	}
	if done {
		return nil
	}

	if settings.Quick {
		color("clean", 32)
	}
	return nil
}
