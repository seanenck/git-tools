// Package cli handles CLI helpers
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsYes will indicate if an environment setting is set to yes
func IsYes(in string) bool {
	if val := strings.ToLower(in); val != "" {
		return val == "yes" || val == "true" || val == "1"
	}
	return false
}

// Fatal is a fatal exit handler
func Fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func gitText(args ...string) string {
	out, _ := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out))
}

// GitRepoOutputText gets output text for a specific repository
func GitRepoOutputText(path string, args ...string) string {
	arguments := []string{"-C", path}
	arguments = append(arguments, args...)
	return gitText(arguments...)
}

// GitRepoBoolConfigValue gets a boolean value for a repository
func GitRepoBoolConfigValue(path, key string) bool {
	return gitToBool(GitRepoOutputText(path, "config", key))
}

// GitConfigValue gets a configuration setting
func GitConfigValue(key string) string {
	return gitText("config", key)
}

// GitBoolConfigValue gets a configuration setting as a bool
func GitBoolConfigValue(key string) bool {
	return gitToBool(GitConfigValue(key))
}

func gitToBool(value string) bool {
	if value == "" {
		return true
	}
	return IsYes(value)
}
