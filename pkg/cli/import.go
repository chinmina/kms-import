package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/chinmina/kms-import/pkg/kmsimport"
)

// runImport decodes the PEM key material via the library, imports it into the
// target KMS key through the injected client, and writes a human-readable
// confirmation to out. alias is the normalised alias string (e.g. "alias/my-key")
// when the caller used --alias, or empty when --key-id/--key-arn was used; it
// appears in the confirmation when non-empty (R25).
func runImport(ctx context.Context, out io.Writer, client kmsimport.KMSClient, keyID, alias string, pemBytes []byte) error {
	der, err := kmsimport.KeyMaterialFromPEM(pemBytes)
	if err != nil {
		return err
	}

	res, err := kmsimport.Import(ctx,
		kmsimport.WithClient(client),
		kmsimport.WithKeyID(keyID),
		kmsimport.WithKeyMaterial(der),
	)
	if err != nil {
		return err
	}

	if alias != "" {
		if _, err := fmt.Fprintf(out, "Imported key material — alias: %s, key ID: %s, state: %s\n", alias, res.KeyID, res.KeyState); err != nil {
			return fmt.Errorf("write confirmation: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(out, "Imported key material — key ID: %s, state: %s\n", res.KeyID, res.KeyState); err != nil {
			return fmt.Errorf("write confirmation: %w", err)
		}
	}
	return nil
}
