package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/chinmina/kms-import/pkg/kmsimport"
)

// runImport decodes the PEM key material via the library, imports it into the
// target KMS key through the injected client, and writes a human-readable
// confirmation to out. alias is the normalised alias string (e.g. "alias/my-key")
// when the caller used --alias, or empty when --key-id/--key-arn was used; it
// appears in the confirmation when non-empty (R25). A non-zero expiry sets the
// imported material to expire at that time (R17); the zero value leaves it
// non-expiring.
func runImport(ctx context.Context, out io.Writer, client kmsimport.KMSClient, keyID, alias string, pemBytes []byte, expiry time.Time) error {
	der, err := kmsimport.KeyMaterialFromPEM(pemBytes)
	if err != nil {
		return err
	}

	importOpts := []kmsimport.Option{
		kmsimport.WithClient(client),
		kmsimport.WithKeyID(keyID),
		kmsimport.WithKeyMaterial(der),
	}
	if !expiry.IsZero() {
		importOpts = append(importOpts, kmsimport.WithExpiry(expiry))
	}

	res, err := kmsimport.Import(ctx, importOpts...)
	if err != nil {
		return err
	}

	aliasPrefix := ""
	if alias != "" {
		aliasPrefix = fmt.Sprintf("alias: %s, ", alias)
	}
	if _, err := fmt.Fprintf(out, "Imported key material — %skey ID: %s, state: %s\n", aliasPrefix, res.KeyID, res.KeyState); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}
	return nil
}
