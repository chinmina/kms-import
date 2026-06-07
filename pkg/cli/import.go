package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/chinmina/kms-import/pkg/kmsimport"
)

// runImport decodes the PEM key material via the library, imports it into the
// target KMS key through the injected client, and writes a human-readable
// confirmation to out. It is the testable seam shared by the CLI Action: flag
// parsing and client construction happen above it, the import and output
// happen here. Key-material decoding is the library's job, not the CLI's.
func runImport(ctx context.Context, out io.Writer, client kmsimport.KMSClient, keyID string, pemBytes []byte) error {
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

	if _, err := fmt.Fprintf(out, "Imported key material — key ID: %s, state: %s\n", res.KeyID, res.KeyState); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}
	return nil
}
