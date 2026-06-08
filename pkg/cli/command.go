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
				Name:  "profile",
				Usage: "AWS named profile to use",
			},
			&clipkg.StringFlag{
				Name:  "region",
				Usage: "AWS region to use",
			},
		},
		MutuallyExclusiveFlags: []clipkg.MutuallyExclusiveFlags{
			{
				Required: true,
				Flags: [][]clipkg.Flag{
					{&clipkg.StringFlag{Name: "key-id", Usage: "the target KMS key ID"}},
					{&clipkg.StringFlag{Name: "key-arn", Usage: "the target KMS key ARN"}},
					{&clipkg.StringFlag{Name: "alias", Usage: "the target KMS key alias (alias/ prefix prepended if absent)"}},
				},
			},
		},
		Action: func(ctx context.Context, cmd *clipkg.Command) error {
			keyID, displayAlias := resolveTarget(cmd)

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

			return runImport(ctx, cmd.Writer, kms.NewFromConfig(cfg), keyID, displayAlias, pemBytes)
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

// resolveTarget returns the resolved KMS key identifier and, when --alias was
// used, the normalised alias for display. The framework has already enforced
// exactly one of the three flags is set via MutuallyExclusiveFlags.
func resolveTarget(cmd *clipkg.Command) (keyID, displayAlias string) {
	if raw := cmd.String("alias"); raw != "" {
		normalised := normaliseAlias(raw)
		return normalised, normalised
	}
	if arn := cmd.String("key-arn"); arn != "" {
		return arn, ""
	}
	return cmd.String("key-id"), ""
}
