// Command kms-import imports a GitHub App private key into AWS KMS.
//
// This binary is a thin wrapper: it mounts the mountable command returned by
// pkg/cli and runs it. All flag handling, SDK client construction, and import
// logic live in pkg/cli and pkg/kmsimport.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/chinmina/kms-import/internal/buildinfo"
	"github.com/chinmina/kms-import/pkg/cli"
)

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	cmd := cli.Command()
	cmd.Version = buildinfo.Version
	cmd.Writer = out
	return cmd.Run(context.Background(), args)
}
