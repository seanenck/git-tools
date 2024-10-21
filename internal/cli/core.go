package cli

import (
	"fmt"
	"os"
)

const (
	// IsMessageOfTheDay is the cli flag for motd outputs
	IsMessageOfTheDay = "motd"
	// MessageOfTheDayPrefix prefixes a string for motd formatting
	MessageOfTheDayPrefix = "  "
)

// Fatal is a fatal exit handler
func Fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
