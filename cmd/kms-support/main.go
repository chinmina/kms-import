// Command kms-support provides support operations used only to prepare
// black-box system tests for kms-import. It is intentionally not included in
// GoReleaser, release artifacts, or installation documentation.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/chinmina/kms-import/internal/buildinfo"
)

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	cmd := command()
	cmd.Version = buildinfo.Version
	cmd.Writer = out
	return cmd.Run(context.Background(), args)
}
