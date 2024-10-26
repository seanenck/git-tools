// Package cli handles CLI helpers
package cli

import (
	"fmt"
	"os"
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
