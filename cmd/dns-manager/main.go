// Package main is the entry point for the dns-manager unified binary.
// It delegates all execution logic to the CLI adapter, which handles
// command routing, flag parsing, and the application lifecycle.
package main

import "github.com/ilindan-dev/dns-manager/internal/adapters/cli"

func main() {
	// Execute the root Cobra command. All setup, execution, and error handling
	// (including os.Exit calls on fatal failures) are managed within the cli package.
	cli.Execute()
}
