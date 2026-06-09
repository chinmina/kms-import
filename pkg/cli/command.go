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
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	clipkg "github.com/urfave/cli/v3"
)

// Command returns the kms-import command. It imports a GitHub App private key
// PEM into the target KMS key, resolving AWS credentials via the standard SDK
// chain.
func Command() *clipkg.Command {
	return &clipkg.Command{
		Name:      "kms-import",
		Usage:     "import a GitHub App private key into AWS KMS",
		UsageText: "kms-import --key-file <pem> (--key-id <id> | --key-arn <arn>) [--expires <rfc3339>] [--profile <name>] [--region <name>] [--json]",
		Description: `kms-import pushes a GitHub App RSA private key into an existing AWS KMS key as
non-extractable key material, so the application can sign GitHub App JWTs with
the KMS Sign API and the private key never leaves the KMS HSM boundary.

The target KMS key must already exist with EXTERNAL origin (created by your
infrastructure-as-code). kms-import only imports key material into it; it never
creates, deletes, or rotates keys. It reads the PEM, converts it to PKCS#8 DER,
fetches wrapping parameters from KMS, encrypts the material under the returned
wrapping key, and calls ImportKeyMaterial. The wrapping algorithm is fixed at
RSA_AES_KEY_WRAP_SHA_256 with an RSA_4096 wrapping key and is not configurable.

Target the key with exactly one of --key-id or --key-arn (mutually exclusive,
one required). The KMS import API does not accept aliases, so there is no
--alias flag; for alias-based rotation, import into the new key by ID/ARN, then
repoint the alias.

Credentials and region resolve through the standard AWS SDK chain (environment,
shared config/credentials files, IMDS). --profile and --region override the
corresponding defaults.

By default the imported material does not expire. --expires sets an expiry
(KEY_MATERIAL_EXPIRES + ValidTo) for organisational compliance; it is not a
revocation mechanism — to revoke a compromised key, revoke it in GitHub. The
same invocation performs both the initial import and a reimport after material
has expired or been deleted; AWS requires a reimport to use the same key
material as the original.

The importing principal needs kms:GetParametersForImport and
kms:ImportKeyMaterial in both its IAM policy and the key's resource policy. See
the README for ready-to-use IAM and key-policy fragments.

EXAMPLES:
   Import a key, targeting it by ARN:
     kms-import --key-file app.pem --key-arn arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab

   Import with an expiry and a named profile, machine-readable output:
     kms-import --key-file app.pem --key-id 1234abcd-12ab-34cd-56ef-1234567890ab --expires 2027-01-01T00:00:00Z --profile ops --json`,
		Flags: []clipkg.Flag{
			&clipkg.StringFlag{
				Name:     "key-file",
				Usage:    "path to the PEM-encoded RSA private key (PKCS#1 \"BEGIN RSA PRIVATE KEY\" or PKCS#8 \"BEGIN PRIVATE KEY\")",
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
			&clipkg.StringFlag{
				Name:  "expires",
				Usage: "expiry for the imported key material as an RFC 3339 timestamp (e.g. 2027-01-01T00:00:00Z); omit for non-expiring material",
			},
			&clipkg.BoolFlag{
				Name:  "json",
				Usage: "emit the result as a JSON object and suppress all other output",
			},
		},
		MutuallyExclusiveFlags: []clipkg.MutuallyExclusiveFlags{
			{
				Required: true,
				Flags: [][]clipkg.Flag{
					{&clipkg.StringFlag{Name: "key-id", Usage: "the target KMS key ID"}},
					{&clipkg.StringFlag{Name: "key-arn", Usage: "the target KMS key ARN"}},
				},
			},
		},
		Action: func(ctx context.Context, cmd *clipkg.Command) error {
			keyID := resolveTarget(cmd)

			// Validate --expires before any AWS call, so a malformed or
			// already-passed value fails fast rather than after a round trip.
			var expiry time.Time
			if e := cmd.String("expires"); e != "" {
				var err error
				if expiry, err = parseExpiry(e); err != nil {
					return err
				}
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

			return runImport(ctx, cmd.Writer, kms.NewFromConfig(cfg), keyID, pemBytes, expiry, cmd.Bool("json"))
		},
	}
}

// parseExpiry parses an RFC 3339 / ISO 8601 timestamp from the --expires flag
// and rejects values that are malformed or not in the future. Callers validate
// before any AWS call so bad input fails fast.
func parseExpiry(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --expires %q: expected RFC 3339 timestamp (e.g. 2027-01-01T00:00:00Z): %w", s, err)
	}
	if !t.After(time.Now()) {
		return time.Time{}, fmt.Errorf("invalid --expires %q: expiry is in the past", s)
	}
	return t, nil
}

// resolveTarget returns the KMS key identifier to import into. The framework has
// already enforced via MutuallyExclusiveFlags that exactly one of --key-id /
// --key-arn is set.
func resolveTarget(cmd *clipkg.Command) string {
	if arn := cmd.String("key-arn"); arn != "" {
		return arn
	}
	return cmd.String("key-id")
}
