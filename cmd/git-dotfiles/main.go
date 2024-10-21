package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"text/template"

	"mvdan.cc/sh/v3/shell"
)

var (
	simpleDiff = []byte{1}
	//go:embed completions.bash
	bashShell string
)

type (
	variables struct {
		System struct {
			OS   string
			Arch string
		}
		root string
		home string
		diff struct {
			exe  string
			args []string
		}
	}
	dotfile struct {
		offset string
		path   string
		info   fs.FileInfo
	}
	result struct {
		err  error
		file dotfile
	}
)

func (d dotfile) display() string {
	return strings.TrimPrefix(d.offset, string(os.PathSeparator))
}

func (r *result) errored(err error) result {
	r.err = err
	return *r
}

func (v variables) get(envKey string) string {
	return os.Getenv("GIT_DOTFILES_" + envKey)
}

func (v variables) list() ([]dotfile, error) {
	found := make(map[string]dotfile)
	var keys []string
	for _, opt := range []string{v.System.OS, v.System.Arch, fmt.Sprintf("%s.%s", v.System.OS, v.System.Arch)} {
		path := filepath.Join(v.root, opt)
		if !PathExists(path) {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(b), "\n") {
			t := strings.TrimSpace(line)
			if t == "" {
				continue
			}
			full := filepath.Join(v.root, t)
			items := []string{full}
			if strings.Contains(full, "*") {
				globs, err := filepath.Glob(full)
				if err != nil {
					return nil, err
				}
				items = globs
			}
			for _, item := range items {
				err := filepath.Walk(item, func(p string, info fs.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}
					if _, ok := found[item]; !ok {
						offset := strings.TrimPrefix(p, v.root)
						found[p] = dotfile{path: p, offset: offset, info: info}
						keys = append(keys, p)
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if len(found) == 0 {
		return nil, errors.New("no items matched")
	}
	var results []dotfile
	for _, k := range keys {
		results = append(results, found[k])
	}
	return results, nil
}

func main() {
	Fatal(run())
}

func (v variables) forEach(fxn func(string, []byte, dotfile) error) error {
	list, err := v.list()
	if err != nil {
		return err
	}
	var results []chan result
	for _, item := range list {
		r := make(chan result)
		go func(object dotfile, c chan result) {
			to := filepath.Join(v.home, object.offset)
			processFile(object, to, v, c, fxn)
		}(item, r)
		results = append(results, r)
	}
	for _, item := range results {
		r := <-item
		if r.err != nil {
			return r.err
		}
	}
	return nil
}

func processFile(item dotfile, to string, v any, c chan result, fxn func(string, []byte, dotfile) error) {
	r := &result{file: item}
	b, err := os.ReadFile(item.path)
	if err != nil {
		c <- r.errored(err)
		return
	}
	s := string(b)
	if strings.Contains(s, "}}") && strings.Contains(s, "{{ $.System.") {
		t, err := template.New("t").Parse(string(b))
		if err != nil {
			c <- r.errored(err)
			return
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, v); err != nil {
			c <- r.errored(err)
			return
		}
		b = buf.Bytes()
	}
	if err := fxn(to, b, item); err != nil {
		c <- r.errored(err)
		return
	}
	c <- *r
}

func diffing(vars variables, verbose bool) (bool, error) {
	type diffResult struct {
		item dotfile
		res  []byte
	}
	var results []diffResult
	err := vars.forEach(func(to string, contents []byte, file dotfile) error {
		res := diffResult{item: file}
		if PathExists(to) {
			r, err := vars.different(to, contents, verbose)
			if err != nil {
				return err
			}
			res.res = r
		} else {
			res.res = simpleDiff
			if verbose {
				res.res = []byte("does not exist")
			}
		}
		if len(res.res) > 0 {
			results = append(results, res)
		}
		return nil
	})
	slices.SortFunc(results, func(x, y diffResult) int {
		return strings.Compare(x.item.offset, y.item.offset)
	})
	differences := false
	for _, item := range results {
		differences = true
		fmt.Printf("-> %s\n", item.item.display())
		if verbose {
			fmt.Println(string(item.res))
		}
	}
	return differences, err
}

func deploy(vars variables, dryRun, overwrite, force bool) error {
	type deployResult struct {
		item     dotfile
		exists   bool
		contents []byte
	}
	var results []deployResult
	err := vars.forEach(func(to string, contents []byte, file dotfile) error {
		exists := false
		if !force {
			exists = PathExists(to)
			if exists {
				r, err := vars.different(to, contents, false)
				if err != nil {
					return err
				}
				if len(r) == 0 {
					return nil
				}
			}
		}
		results = append(results, deployResult{item: file, exists: exists, contents: contents})
		return nil
	})
	if err != nil {
		return err
	}
	slices.SortFunc(results, func(x, y deployResult) int {
		return strings.Compare(x.item.offset, y.item.offset)
	})
	for _, item := range results {
		status := "adding"
		if item.exists {
			status = "differs"
		}
		if force {
			status = ""
		} else {
			status = fmt.Sprintf(" (%s)", status)
		}
		fmt.Printf("-> %s%s\n", item.item.display(), status)
		if !force {
			if dryRun {
				continue
			}
			if item.exists && !overwrite {
				fmt.Println("    ^ skipped")
				continue
			}
		}
		h := filepath.Join(vars.home, item.item.offset)
		dir := filepath.Dir(h)
		if !PathExists(dir) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		if err := os.WriteFile(h, item.contents, item.item.info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func (v variables) different(file string, b []byte, verbose bool) ([]byte, error) {
	f, err := os.CreateTemp("", "dotfiles.")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		return nil, err
	}
	return v.doDiff(file, f.Name(), verbose), nil
}

func (v variables) doDiff(left, right string, verbose bool) []byte {
	args := v.diff.args
	args = append(args, left, right)
	cmd := exec.Command(v.diff.exe, args...)
	if !verbose {
		if err := cmd.Run(); err != nil {
			return simpleDiff
		}
		return nil
	}
	b, _ := cmd.CombinedOutput()
	return b
}

func run() error {
	args := os.Args
	if len(args) < 2 {
		return errors.New("command required")
	}
	vars := variables{}
	vars.System.OS = runtime.GOOS
	vars.System.Arch = runtime.GOARCH
	vars.root = vars.get("ROOT")
	if vars.root == "" {
		return errors.New("dotfiles root not set")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	vars.home = home
	diff := vars.get("DIFF")
	if diff == "" {
		diff = "diff -u"
	}
	fields, err := shell.Fields(diff, os.Getenv)
	if err != nil {
		return err
	}
	if len(fields) == 0 {
		return errors.New("unable to determine diff utility")
	}
	vars.diff.exe = fields[0]
	if len(fields) > 1 {
		vars.diff.args = fields[1:]
	}
	arguments := struct {
		Deploy string
		Diff   string
		Args   struct {
			Verbose   string
			DryRun    string
			Force     string
			Overwrite string
		}
	}{}
	arguments.Deploy = "deploy"
	arguments.Diff = "diff"
	arguments.Args.DryRun = "--dry-run"
	arguments.Args.Force = "--force"
	arguments.Args.Verbose = "--verbose"
	arguments.Args.Overwrite = "--overwrite"

	count := len(args)
	switch args[1] {
	case "completions":
		t, err := template.New("c").Parse(bashShell)
		if err != nil {
			return err
		}
		return t.Execute(os.Stdout, arguments)
	case arguments.Diff:
		verbose := false
		if count <= 3 {
			if count == 3 {
				if strings.ToLower(args[2]) == arguments.Args.Verbose {
					verbose = true
				} else {
					return errors.New("unknown argument for diff")
				}
			}
			had, err := diffing(vars, verbose)
			if err != nil {
				return err
			}
			if had {
				os.Exit(1)
			}
			return nil
		}
	case arguments.Deploy:
		dryRun := false
		force := false
		overwrite := false
		if count <= 3 {
			if count == 3 {
				switch strings.ToLower(args[2]) {
				case arguments.Args.DryRun:
					dryRun = true
				case arguments.Args.Force:
					force = true
				case arguments.Args.Overwrite:
					overwrite = true
				default:
					return errors.New("unknown argument for deploy")
				}
			}
			return deploy(vars, dryRun, overwrite, force)
		}
	}
	return errors.New("invalid arguments")
}
