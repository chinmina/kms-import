package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// fakeKMS is a KMSClient that returns a real wrapping public key so the import
// crypto succeeds, then reports the key as resolved.
type fakeKMS struct {
	keyID string
}

func (f fakeKMS) GetParametersForImport(_ context.Context, _ *kms.GetParametersForImportInput, _ ...func(*kms.Options)) (*kms.GetParametersForImportOutput, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, err
	}
	return &kms.GetParametersForImportOutput{
		KeyId:       &f.keyID,
		PublicKey:   der,
		ImportToken: []byte("token"),
	}, nil
}

func (f fakeKMS) ImportKeyMaterial(_ context.Context, _ *kms.ImportKeyMaterialInput, _ ...func(*kms.Options)) (*kms.ImportKeyMaterialOutput, error) {
	return &kms.ImportKeyMaterialOutput{KeyId: &f.keyID}, nil
}

func TestRunImport_PrintsConfirmation(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyID := "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"

	var out bytes.Buffer
	err = runImport(context.Background(), &out, fakeKMS{keyID: keyID}, keyID, pkcs1PEM(t, priv))
	if err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, keyID) {
		t.Errorf("confirmation %q does not contain key ID %q", got, keyID)
	}
	if !strings.Contains(got, "Enabled") {
		t.Errorf("confirmation %q does not contain key state %q", got, "Enabled")
	}
}
