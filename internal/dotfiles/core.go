// Package dotfiles wraps around deploying dotfiles
package dotfiles

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"text/template"

	"github.com/seanenck/git-tools/internal/paths"
	"mvdan.cc/sh/v3/shell"
)

var (
	simpleDiff = []byte{1}
	//go:embed completions.bash
	bashShell string
)

type (
	variables struct {
		Dotfiles struct {
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
	// Settings are dotfiles arguments/input settngs
	Settings struct {
		Mode      string
		Overwrite bool
		Force     bool
		DryRun    bool
		Verbose   bool
		Writer    io.Writer
	}
	templating struct {
		re     *regexp.Regexp
		fields []string
		object any
	}
)

const (
	// CompletionsMode indicates to generate completions
	CompletionsMode = "completions"
	// DeployMode indicates file deployment
	DeployMode = "deploy"
	// DiffMode indicates showing a diff
	DiffMode = "diff"
	// DryRunArg is the argumnent to outline changes (not make them)
	DryRunArg = "dry-run"
	// ForceArg will force write all files
	ForceArg = "force"
	// VerboseArg will enable verbose diffing
	VerboseArg = "verbose"
	// OverwriteArg will overwrite differing files
	OverwriteArg = "overwrite"
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
	for _, opt := range []string{v.Dotfiles.OS, v.Dotfiles.Arch, fmt.Sprintf("%s.%s", v.Dotfiles.OS, v.Dotfiles.Arch)} {
		path := filepath.Join(v.root, opt)
		if !paths.Exists(path) {
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

func (v variables) forEach(fxn func(string, []byte, dotfile) error) error {
	list, err := v.list()
	if err != nil {
		return err
	}
	r, err := regexp.Compile(`{{(.*?)}}`)
	if err != nil {
		return err
	}
	t := templating{re: r, object: v}
	fields := reflect.ValueOf(v.Dotfiles)
	for i := 0; i < fields.NumField(); i++ {
		t.fields = append(t.fields, fmt.Sprintf("$.Dotfiles.%s", fields.Type().Field(i).Name))
	}
	var results []chan result
	for _, item := range list {
		r := make(chan result)
		go func(object dotfile, c chan result) {
			to := filepath.Join(v.home, object.offset)
			processFile(object, to, t, c, fxn)
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

func doTemplate(in string, v any) ([]byte, error) {
	t, err := template.New("t").Parse(in)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func isTemplated(s string, t templating) (bool, error) {
	if s == "" {
		return false, nil
	}

	matches := t.re.FindAllStringSubmatch(s, -1)
	m := len(matches)
	if m == 0 {
		return false, nil
	}
	count := 0
	for _, v := range matches {
		val := strings.TrimSpace(v[1])
		if !slices.Contains(t.fields, val) {
			continue
		}
		count++
	}
	if count > 0 {
		if count != m {
			return false, errors.New("can not mix dotfiles and non-dotfiles templating")
		}
		return true, nil
	}
	return false, nil
}

func processFile(item dotfile, to string, t templating, c chan result, fxn func(string, []byte, dotfile) error) {
	r := &result{file: item}
	b, err := os.ReadFile(item.path)
	if err != nil {
		c <- r.errored(err)
		return
	}
	s := string(b)
	is, err := isTemplated(s, t)
	if err != nil {
		c <- r.errored(err)
		return
	}
	if is {
		t, err := doTemplate(s, t.object)
		if err != nil {
			c <- r.errored(err)
			return
		}
		b = t
	}
	if err := fxn(to, b, item); err != nil {
		c <- r.errored(err)
		return
	}
	c <- *r
}

func diffing(vars variables, s Settings) error {
	type diffResult struct {
		item dotfile
		res  []byte
	}
	var results []diffResult
	err := vars.forEach(func(to string, contents []byte, file dotfile) error {
		res := diffResult{item: file}
		if paths.Exists(to) {
			r, err := vars.different(to, contents, s.Verbose)
			if err != nil {
				return err
			}
			res.res = r
		} else {
			res.res = simpleDiff
			if s.Verbose {
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
	for _, item := range results {
		fmt.Fprintf(s.Writer, "-> %s\n", item.item.display())
		if s.Verbose {
			fmt.Fprintln(s.Writer, string(item.res))
		}
	}
	return err
}

func deploy(vars variables, s Settings) error {
	type deployResult struct {
		item     dotfile
		exists   bool
		contents []byte
	}
	var results []deployResult
	err := vars.forEach(func(to string, contents []byte, file dotfile) error {
		exists := false
		if !s.Force {
			exists = paths.Exists(to)
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
	has := false
	for _, item := range results {
		status := "adding"
		if item.exists {
			status = "differs"
		}
		if s.Force {
			status = ""
		} else {
			status = fmt.Sprintf(" (%s)", status)
		}
		fmt.Fprintf(s.Writer, "-> %s%s\n", item.item.display(), status)
		if !s.Force {
			if item.exists && !s.Overwrite {
				fmt.Fprintln(s.Writer, "    ^ skipped")
				continue
			}
		}
		has = true
		if s.DryRun {
			continue
		}
		h := filepath.Join(vars.home, item.item.offset)
		dir := filepath.Dir(h)
		if !paths.Exists(dir) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		if err := os.WriteFile(h, item.contents, item.item.info.Mode()); err != nil {
			return err
		}
	}
	if s.DryRun && has {
		fmt.Fprintln(s.Writer, "\n[DRYRUN] no changes made")
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

// Do will execute dotfiles activities
func Do(s Settings) error {
	if s.Writer == nil {
		return errors.New("writer is nil")
	}
	vars := variables{}
	vars.Dotfiles.OS = runtime.GOOS
	vars.Dotfiles.Arch = runtime.GOARCH
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
		Exe string
	}{}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	arguments.Exe = filepath.Base(exe)
	arguments.Deploy = DeployMode
	arguments.Diff = DiffMode
	arguments.Args.Force = fmt.Sprintf("--%s", ForceArg)
	arguments.Args.DryRun = fmt.Sprintf("--%s", DryRunArg)
	arguments.Args.Verbose = fmt.Sprintf("--%s", VerboseArg)
	arguments.Args.Overwrite = fmt.Sprintf("--%s", OverwriteArg)

	switch s.Mode {
	case CompletionsMode:
		t, err := template.New("c").Parse(bashShell)
		if err != nil {
			return err
		}
		return t.Execute(os.Stdout, arguments)
	case arguments.Diff:
		return diffing(vars, s)
	case arguments.Deploy:
		return deploy(vars, s)
	}
	return errors.New("invalid arguments")
}
