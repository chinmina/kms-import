package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// pkcs1PEM returns a PKCS#1 ("BEGIN RSA PRIVATE KEY") PEM encoding of priv,
// GitHub's private key output format, for driving the end-to-end import.
func pkcs1PEM(t *testing.T, priv *rsa.PrivateKey) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
}

// wrappingKeyDER is a wrapping public key (SubjectPublicKeyInfo DER), the shape
// GetParametersForImport returns. The library requests WrappingKeySpec RSA_4096,
// so KMS always returns a 4096-bit key — match that here. Generated once for the
// whole test binary to keep the cost to a single keygen.
var wrappingKeyDER = sync.OnceValue(func() []byte {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		panic(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		panic(err)
	}
	return der
})

// fakeKMS is a KMSClient that returns a real wrapping public key so the import
// crypto succeeds, then reports the key as resolved.
type fakeKMS struct {
	keyID string
}

func (f fakeKMS) GetParametersForImport(_ context.Context, _ *kms.GetParametersForImportInput, _ ...func(*kms.Options)) (*kms.GetParametersForImportOutput, error) {
	return &kms.GetParametersForImportOutput{
		KeyId:       &f.keyID,
		PublicKey:   wrappingKeyDER(),
		ImportToken: []byte("token"),
	}, nil
}

func (f fakeKMS) ImportKeyMaterial(_ context.Context, _ *kms.ImportKeyMaterialInput, _ ...func(*kms.Options)) (*kms.ImportKeyMaterialOutput, error) {
	return &kms.ImportKeyMaterialOutput{KeyId: &f.keyID}, nil
}

// recordingKMS wraps fakeKMS to additionally capture the ImportKeyMaterial
// input, so tests can assert on the expiry fields runImport forwards through the
// library.
type recordingKMS struct {
	fakeKMS
	ikmInput *kms.ImportKeyMaterialInput
}

func (r *recordingKMS) ImportKeyMaterial(ctx context.Context, in *kms.ImportKeyMaterialInput, optFns ...func(*kms.Options)) (*kms.ImportKeyMaterialOutput, error) {
	r.ikmInput = in
	return r.fakeKMS.ImportKeyMaterial(ctx, in, optFns...)
}

// TestRunImport_SetsExpiry checks that a non-zero expiry passed to runImport
// reaches the import as KEY_MATERIAL_EXPIRES with ValidTo set (R17).
func TestRunImport_SetsExpiry(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyID := "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"
	expiry := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)

	rec := &recordingKMS{fakeKMS: fakeKMS{keyID: keyID}}
	var out bytes.Buffer
	if err := runImport(context.Background(), &out, rec, keyID, pkcs1PEM(t, priv), expiry, false); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	if got := rec.ikmInput.ExpirationModel; got != types.ExpirationModelTypeKeyMaterialExpires {
		t.Errorf("ExpirationModel = %q, want KEY_MATERIAL_EXPIRES", got)
	}
	if got := aws.ToTime(rec.ikmInput.ValidTo); !got.Equal(expiry) {
		t.Errorf("ValidTo = %v, want %v", got, expiry)
	}
}

// failingKMS fails GetParametersForImport, standing in for any AWS API failure.
type failingKMS struct{}

func (failingKMS) GetParametersForImport(context.Context, *kms.GetParametersForImportInput, ...func(*kms.Options)) (*kms.GetParametersForImportOutput, error) {
	return nil, errors.New("boom")
}

func (failingKMS) ImportKeyMaterial(context.Context, *kms.ImportKeyMaterialInput, ...func(*kms.Options)) (*kms.ImportKeyMaterialOutput, error) {
	return nil, errors.New("boom")
}

// TestRunImport_FailureWritesNothing checks that an AWS failure surfaces as an
// error and leaves stdout empty, so quiet/JSON mode never emits partial output
// (R27, and the R26 suppression guarantee).
func TestRunImport_FailureWritesNothing(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var out bytes.Buffer
	err = runImport(context.Background(), &out, failingKMS{}, "key-id", pkcs1PEM(t, priv), time.Time{}, true)
	if err == nil {
		t.Fatal("runImport with a failing client succeeded, want error")
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty on failure, got %q", out.String())
	}
}

// TestRunImport_JSONOutput checks that with jsonOut=true, runImport writes a
// single valid JSON object carrying the key ID and state, and suppresses the
// human-readable confirmation line (R26).
func TestRunImport_JSONOutput(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyID := "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"

	var out bytes.Buffer
	if err := runImport(context.Background(), &out, fakeKMS{keyID: keyID}, keyID, pkcs1PEM(t, priv), time.Time{}, true); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	var got struct {
		KeyID    string `json:"keyId"`
		KeyState string `json:"keyState"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if got.KeyID != keyID {
		t.Errorf("keyId = %q, want %q", got.KeyID, keyID)
	}
	if got.KeyState != "Enabled" {
		t.Errorf("keyState = %q, want %q", got.KeyState, "Enabled")
	}
	if strings.Contains(out.String(), "Imported key material") {
		t.Errorf("JSON output should suppress the human-readable line, got %q", out.String())
	}
}

func TestRunImport_PrintsConfirmation(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyID := "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"

	var out bytes.Buffer
	err = runImport(context.Background(), &out, fakeKMS{keyID: keyID}, keyID, pkcs1PEM(t, priv), time.Time{}, false)
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
