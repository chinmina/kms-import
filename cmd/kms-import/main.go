// Command kms-import imports a GitHub App private key into AWS KMS.
//
// This is the Phase 1 tracer-bullet build: it wires up the binary, version
// stamping, and the release pipeline end to end. The import flags and logic are
// added in later phases (the CLI is rebuilt on urfave/cli v3 in Phase 4).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/chinmina/kms-import/internal/buildinfo"
)

const name = "kms-import"

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) > 1 {
		switch args[1] {
		case "version", "--version", "-v":
			return buildinfo.Fprint(out, name)
		}
	}
	return usage(out)
}

func usage(out io.Writer) error {
	_, err := fmt.Fprintf(out, `%s %s

Import a GitHub App private key into AWS KMS.

Usage:
  %s --key-file <path> (--key-id <id> | --key-arn <arn> | --alias <name>) [flags]

Run "%s version" to print the version.

Import flags are added in later phases; see docs/plan-kms-import.md.
`, name, buildinfo.Version, name, name)
	if err != nil {
		return fmt.Errorf("write usage: %w", err)
	}
	return nil
}
