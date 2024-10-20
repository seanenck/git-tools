// Package main handles various git uncommitted states for output
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/shell"
)

func main() {
	Fatal(gitUncommitted())
}

func uncommit(stdout chan string, dir string) {
	cmd := exec.Command("git", "current-state")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err == nil {
		res := strings.TrimSpace(string(out))
		if res != "" {
			stdout <- res
			return
		}
	}
	stdout <- ""
}

func gitUncommitted() error {
	mode := flag.String("mode", "", "operating mode")
	flag.Parse()
	op := *mode
	if op == "pwd" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		state, _ := exec.Command("git", "-C", wd, "rev-parse", "--is-inside-work-tree").Output()
		if strings.TrimSpace(string(state)) == "true" {
			cmd := exec.Command("git", "current-state", "--quick")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return nil
	}
	const key = "GIT_UNCOMMITTED"
	in := strings.TrimSpace(os.Getenv(key))
	if in == "" {
		return nil
	}
	dirs, err := shell.Fields(in, os.Getenv)
	if err != nil {
		return err
	}
	home := os.Getenv("HOME")
	if val := strings.ToLower(os.Getenv(key + "_HOME")); val != "" {
		if val == "yes" || val == "1" || val == "true" {
			if home == "" {
				return errors.New("unable to process HOME, not set?")
			}
			dirs = append(dirs, filepath.Dir(home))
		}
	}

	var wg sync.WaitGroup
	var all []chan string
	for _, dir := range dirs {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			children, err := os.ReadDir(path)
			if err != nil {
				return
			}
			for _, child := range children {
				childPath := filepath.Join(path, child.Name())
				if !PathExists(filepath.Join(childPath, ".git")) {
					continue
				}
				r := make(chan string)
				go func(dir string, out chan string) {
					uncommit(out, dir)
				}(childPath, r)
				all = append(all, r)
			}
		}(dir)
	}
	wg.Wait()
	var results []string
	prefix := ""
	isMessage := op == "motd"
	if isMessage {
		prefix = "  "
	}
	for _, a := range all {
		res := <-a
		if res != "" {
			for _, line := range strings.Split(res, "\n") {
				results = append(results, fmt.Sprintf("%s%s", prefix, strings.Replace(line, home, "~", 1)))
			}
		}
	}
	if len(results) > 0 {
		if isMessage {
			fmt.Println("uncommitted\n===")
		}
		sort.Strings(results)
		fmt.Println(strings.Join(results, "\n"))
	}
	return nil
}
