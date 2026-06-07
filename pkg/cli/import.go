package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/chinmina/kms-import/pkg/kmsimport"
)

// runImport decodes the PEM key material, imports it into the target KMS key via
// the injected client, and writes a human-readable confirmation to out. It is
// the testable seam shared by the CLI Action: client construction and flag
// parsing happen above it, the import and output happen here.
func runImport(ctx context.Context, out io.Writer, client kmsimport.KMSClient, keyID string, pemBytes []byte) error {
	der, err := decodeKeyMaterial(pemBytes)
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

	if _, err := fmt.Fprintf(out, "Imported key material — key ID: %s, state: %s\n", res.KeyID, res.KeyState); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}
	return nil
}
