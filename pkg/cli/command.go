// Package cli provides the kms-import command as a mountable urfave/cli v3
// command. Command() returns a *cli.Command so it can run standalone (via the
// thin cmd/kms-import wrapper) or be mounted as a subcommand of another
// urfave/cli application such as chinmina-bridge.
//
// SDK client construction, credential/region resolution, and exit-code mapping
// live in this layer; the kmsimport library never builds an SDK client.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	clipkg "github.com/urfave/cli/v3"
)

// Command returns the kms-import command. It imports a GitHub App private key
// PEM into the target KMS key, resolving AWS credentials via the standard SDK
// chain.
func Command() *clipkg.Command {
	return &clipkg.Command{
		Name:  "kms-import",
		Usage: "import a GitHub App private key into AWS KMS",
		Flags: []clipkg.Flag{
			&clipkg.StringFlag{
				Name:     "key-file",
				Usage:    "path to the PEM-encoded private key file",
				Required: true,
			},
			&clipkg.StringFlag{
				Name:     "key-id",
				Usage:    "the target KMS key identifier",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *clipkg.Command) error {
			pemBytes, err := os.ReadFile(cmd.String("key-file"))
			if err != nil {
				return fmt.Errorf("read key file: %w", err)
			}

			cfg, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				return fmt.Errorf("load AWS config: %w", err)
			}

			return runImport(ctx, cmd.Writer, kms.NewFromConfig(cfg), cmd.String("key-id"), pemBytes)
		},
	}
}
