package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/chinmina/kms-import/pkg/kmsimport"
)

// runImport decodes the PEM key material via the library, imports it into the
// target KMS key through the injected client, and writes a human-readable
// confirmation to out. A non-zero expiry sets the imported material to expire at
// that time; the zero value leaves it non-expiring.
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

	return writeConfirmation(out, res.KeyID, res.KeyState)
}

// writeConfirmation renders the human-readable success summary. When the key
// ends up Enabled the material is usable immediately, so say so explicitly;
// any other state means the key still needs attention before it can sign.
func writeConfirmation(out io.Writer, keyID, keyState string) error {
	var b strings.Builder

	b.WriteString("✅ Key material imported successfully\n")
	fmt.Fprintf(&b, " - Key ID: %s\n", keyID)
	fmt.Fprintf(&b, " - State:  %s\n", keyState)

	if strings.EqualFold(keyState, string(types.KeyStateEnabled)) {
		b.WriteString("\nThe key is ready to use for signing.\n")
	} else {
		fmt.Fprintf(&b, "\nThe key is not enabled (state: %s) and cannot be used for signing yet.\n", keyState)
	}

	if _, err := io.WriteString(out, b.String()); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}
	return nil
}

// importResult is the JSON shape emitted under --json: the resolved key ID and
// the resulting key state. These field names are the documented machine-readable
// contract — keep them stable.
type importResult struct {
	KeyID    string `json:"keyId"`
	KeyState string `json:"keyState"`
}

// writeJSON emits the machine-readable import result as the sole output under
// --json. Encode writes directly to out and appends a trailing newline.
func writeJSON(out io.Writer, keyID, keyState string) error {
	if err := json.NewEncoder(out).Encode(importResult{KeyID: keyID, KeyState: keyState}); err != nil {
		return fmt.Errorf("write JSON result: %w", err)
	}
	return nil
}
