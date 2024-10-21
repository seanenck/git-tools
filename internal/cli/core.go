// Package cli handles CLI helpers
package cli

import (
	"fmt"
	"os"
)

// Fatal is a fatal exit handler
func Fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
