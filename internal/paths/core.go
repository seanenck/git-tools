// Package paths handle filepathing
package paths

import (
	"errors"
	"os"
)

// Exists indicate if a path exists
func Exists(file string) bool {
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}
