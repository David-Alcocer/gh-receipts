// Command gh-receipts is a GitHub CLI extension that reports what a team
// actually shipped, using facts from Git and GitHub.
package main

import (
	"fmt"
	"os"

	"github.com/David-Alcocer/gh-receipts/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
