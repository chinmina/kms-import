package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/chinmina/kms-import/pkg/kmsimport"
)

// runImport decodes the PEM key material via the library, imports it into the
// target KMS key through the injected client, and writes a human-readable
// confirmation to out. A non-zero expiry sets the imported material to expire at
// that time (R17); the zero value leaves it non-expiring.
func runImport(ctx context.Context, out io.Writer, client kmsimport.KMSClient, keyID string, pemBytes []byte, expiry time.Time, jsonOut bool) error {
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

	if jsonOut {
		return writeJSON(out, res.KeyID, res.KeyState)
	}

	if _, err := fmt.Fprintf(out, "Imported key material — key ID: %s, state: %s\n", res.KeyID, res.KeyState); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}
	return nil
}

// importResult is the JSON shape emitted under --json (R26): the resolved key ID
// and the resulting key state.
type importResult struct {
	KeyID    string `json:"keyId"`
	KeyState string `json:"keyState"`
}

// writeJSON emits the machine-readable import result, suppressing all other
// output (R26). Encode writes directly to out and appends a trailing newline.
func writeJSON(out io.Writer, keyID, keyState string) error {
	if err := json.NewEncoder(out).Encode(importResult{KeyID: keyID, KeyState: keyState}); err != nil {
		return fmt.Errorf("write JSON result: %w", err)
	}
	return nil
}
