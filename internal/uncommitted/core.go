// Package uncommitted handles various git uncommitted states for output
package uncommitted

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/paths"
	"github.com/seanenck/git-tools/internal/state"
	"mvdan.cc/sh/v3/shell"
)

// Settings are the operating settings for this mode
type Settings struct {
	Mode   string
	Writer io.Writer
}

func stateSettings(dir string, quick bool, w io.Writer) state.Settings {
	return state.Settings{
		Branches: state.DefaultBranches,
		Writer:   w,
		Dir:      dir,
		Quick:    quick,
	}
}

func uncommit(stdout chan string, dir string) {
	var buf bytes.Buffer
	if err := state.Current(stateSettings(dir, false, &buf)); err == nil {
		res := strings.TrimSpace(buf.String())
		if res != "" {
			stdout <- res
			return
		}
	}
	stdout <- ""
}

func gitGetText(path string, args ...string) string {
	arguments := []string{"-C", path}
	arguments = append(arguments, args...)
	out, _ := exec.Command("git", arguments...).Output()
	return strings.TrimSpace(string(out))
}

func isWorkTree() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if gitGetText(wd, "rev-parse", "--is-inside-work-tree") == "true" {
		return wd, nil
	}
	return "", nil
}

// Current will get uncommitted state information
func Current(s Settings) error {
	if s.Writer == nil {
		return errors.New("writer is nil")
	}
	const (
		key          = "GIT_UNCOMMITTED"
		baseConfig   = "uncommitted."
		isPrompt     = "prompt"
		isScan       = "scan"
		configMode   = "config."
		promptConfig = baseConfig + isPrompt
		scanConfig   = baseConfig + isScan
		isPromptMode = configMode + isPrompt
		isScanMode   = configMode + isScan
	)
	isConfigOn := func(path, key string) bool {
		value := gitGetText(path, "config", key)
		if value == "" {
			return true
		}
		return cli.IsYes(value)
	}
	switch s.Mode {
	case isPromptMode, isScanMode:
		wd, err := isWorkTree()
		if err != nil {
			return err
		}
		if wd == "" {
			return nil
		}
		useKey := promptConfig
		if s.Mode == isScanMode {
			useKey = scanConfig
		}
		value := fmt.Sprintf("%t", !isConfigOn(wd, useKey))
		fmt.Printf("setting: %s = %s\n", useKey, value)
		return exec.Command("git", "-C", wd, "config", useKey, value).Run()
	case "pwd":
		if cli.IsYes(os.Getenv(key + "_NO_PROMPT")) {
			return nil
		}
		wd, err := isWorkTree()
		if err != nil {
			return err
		}
		if wd != "" {
			if !isConfigOn(wd, promptConfig) {
				return nil
			}
			set := stateSettings(wd, true, s.Writer)
			return state.Current(set)
		}
		return nil
	case "":
	default:
		return fmt.Errorf("unknown mode: %s", s.Mode)
	}
	if cli.IsYes(os.Getenv(key + "_NO_SCAN")) {
		return nil
	}
	in := strings.TrimSpace(os.Getenv(key))
	if in == "" {
		return nil
	}
	dirs, err := shell.Fields(in, os.Getenv)
	if err != nil {
		return err
	}
	home := os.Getenv("HOME")
	if cli.IsYes(os.Getenv(key + "_HOME")) {
		if home == "" {
			return errors.New("unable to process HOME, not set?")
		}
		dirs = append(dirs, filepath.Dir(home))
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
				if !paths.Exists(filepath.Join(childPath, ".git")) {
					continue
				}
				if !isConfigOn(childPath, scanConfig) {
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
	for _, a := range all {
		res := <-a
		if res != "" {
			for _, line := range strings.Split(res, "\n") {
				results = append(results, strings.Replace(line, home, "~", 1))
			}
		}
	}
	if len(results) > 0 {
		sort.Strings(results)
		fmt.Fprintln(s.Writer, strings.Join(results, "\n"))
	}
	return nil
}
