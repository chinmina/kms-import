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
	"strings"

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
				Name:  "key-id",
				Usage: "the target KMS key ID",
			},
			&clipkg.StringFlag{
				Name:  "key-arn",
				Usage: "the target KMS key ARN",
			},
			&clipkg.StringFlag{
				Name:  "alias",
				Usage: "the target KMS key alias (alias/ prefix prepended if absent)",
			},
			&clipkg.StringFlag{
				Name:  "profile",
				Usage: "AWS named profile to use",
			},
			&clipkg.StringFlag{
				Name:  "region",
				Usage: "AWS region to use",
			},
		},
		Action: func(ctx context.Context, cmd *clipkg.Command) error {
			keyID, err := resolveKeyID(cmd)
			if err != nil {
				return err
			}

			pemBytes, err := os.ReadFile(cmd.String("key-file"))
			if err != nil {
				return fmt.Errorf("read key file: %w", err)
			}

			var cfgOpts []func(*config.LoadOptions) error
			if p := cmd.String("profile"); p != "" {
				cfgOpts = append(cfgOpts, config.WithSharedConfigProfile(p))
			}
			if r := cmd.String("region"); r != "" {
				cfgOpts = append(cfgOpts, config.WithRegion(r))
			}

			cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
			if err != nil {
				return fmt.Errorf("load AWS config: %w", err)
			}

			alias := cmd.String("alias")
			return runImport(ctx, cmd.Writer, kms.NewFromConfig(cfg), keyID, alias, pemBytes)
		},
	}
}

// normaliseAlias prepends "alias/" if not already present (R10, R11).
func normaliseAlias(alias string) string {
	if strings.HasPrefix(alias, "alias/") {
		return alias
	}
	return "alias/" + alias
}

// resolveKeyID returns the normalised key identifier from the mutually-exclusive
// target flags --key-id, --key-arn, and --alias (R7–R11).
func resolveKeyID(cmd *clipkg.Command) (string, error) {
	id, arn, alias := cmd.String("key-id"), cmd.String("key-arn"), cmd.String("alias")
	n := 0
	if id != "" {
		n++
	}
	if arn != "" {
		n++
	}
	if alias != "" {
		n++
	}
	if n == 0 {
		return "", fmt.Errorf("exactly one of --key-id, --key-arn, or --alias must be provided")
	}
	if n > 1 {
		return "", fmt.Errorf("--key-id, --key-arn, and --alias are mutually exclusive: provide exactly one")
	}
	if alias != "" {
		return normaliseAlias(alias), nil
	}
	if arn != "" {
		return arn, nil
	}
	return id, nil
}
