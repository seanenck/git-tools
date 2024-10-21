// Package main supports dotfile operations
package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/seanenck/git-tools/internal/cli"
	"github.com/seanenck/git-tools/internal/dotfiles"
)

func main() {
	cli.Fatal(run())
}

func run() error {
	args := os.Args
	if len(args) < 2 {
		return errors.New("command required")
	}
	settings := dotfiles.Settings{}
	settings.Writer = os.Stdout
	settings.Mode = args[1]
	flagSet := flag.NewFlagSet(settings.Mode, flag.ContinueOnError)
	switch settings.Mode {
	case dotfiles.CompletionsMode:
		if len(args) != 2 {
			return errors.New("invalid arguments, none accepted")
		}
	case dotfiles.DiffMode:
		verbose := flagSet.Bool(dotfiles.VerboseArg, false, "enable verbose output (show diff results)")
		if err := flagSet.Parse(args[2:]); err != nil {
			return err
		}
		settings.Verbose = *verbose
	case dotfiles.DeployMode:
		overwrite := flagSet.Bool(dotfiles.OverwriteArg, false, "overwrite files on difference")
		force := flagSet.Bool(dotfiles.ForceArg, false, "overwrite all files")
		dryRun := flagSet.Bool(dotfiles.DryRunArg, false, "perform dry-run (do not make changes)")
		if err := flagSet.Parse(args[2:]); err != nil {
			return err
		}
		settings.DryRun = *dryRun
		settings.Force = *force
		settings.Overwrite = *overwrite
		if settings.Force && settings.Overwrite {
			return errors.New("can not force and overwrite (force implies overwrite)")
		}
	default:
		return fmt.Errorf("unknown mode: %s", settings.Mode)
	}
	return dotfiles.Do(settings)
}
