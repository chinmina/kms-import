package kmsimport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// mockKMS is a test double for KMSClient that records the inputs it receives and
// returns canned responses or errors.
type mockKMS struct {
	gpiInput  *kms.GetParametersForImportInput
	gpiOutput *kms.GetParametersForImportOutput
	gpiErr    error

	ikmInput  *kms.ImportKeyMaterialInput
	ikmOutput *kms.ImportKeyMaterialOutput
	ikmErr    error
}

func (m *mockKMS) GetParametersForImport(_ context.Context, in *kms.GetParametersForImportInput, _ ...func(*kms.Options)) (*kms.GetParametersForImportOutput, error) {
	m.gpiInput = in
	return m.gpiOutput, m.gpiErr
}

func (m *mockKMS) ImportKeyMaterial(_ context.Context, in *kms.ImportKeyMaterialInput, _ ...func(*kms.Options)) (*kms.ImportKeyMaterialOutput, error) {
	m.ikmInput = in
	return m.ikmOutput, m.ikmErr
}

// newWrappingKey returns a fresh RSA key plus its SubjectPublicKeyInfo DER
// encoding, as GetParametersForImport returns in its PublicKey field.
func newWrappingKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate wrapping key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal wrapping public key: %v", err)
	}
	return priv, der
}

func TestImport_Success(t *testing.T) {
	priv, pubDER := newWrappingKey(t)
	token := []byte("import-token-bytes")
	keyARN := "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"
	material := mustHex(t, "3082010203040506") // stand-in PKCS#8 DER

	m := &mockKMS{
		gpiOutput: &kms.GetParametersForImportOutput{
			KeyId:       new(keyARN),
			PublicKey:   pubDER,
			ImportToken: token,
		},
		ikmOutput: &kms.ImportKeyMaterialOutput{
			KeyId: new(keyARN),
		},
	}

	res, err := Import(context.Background(),
		WithClient(m),
		WithKeyID(keyARN),
		WithKeyMaterial(material),
	)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	// Result reflects the resolved key and the post-import state.
	if res.KeyID != keyARN {
		t.Errorf("Result.KeyID = %q, want %q", res.KeyID, keyARN)
	}
	if res.KeyState != "Enabled" {
		t.Errorf("Result.KeyState = %q, want %q", res.KeyState, "Enabled")
	}

	// GetParametersForImport uses the locked wrapping algorithm and key spec.
	if got := m.gpiInput.WrappingAlgorithm; got != types.AlgorithmSpecRsaAesKeyWrapSha256 {
		t.Errorf("GPI WrappingAlgorithm = %q, want RSA_AES_KEY_WRAP_SHA_256", got)
	}
	if got := m.gpiInput.WrappingKeySpec; got != types.WrappingKeySpecRsa4096 {
		t.Errorf("GPI WrappingKeySpec = %q, want RSA_4096", got)
	}
	if got := aws.ToString(m.gpiInput.KeyId); got != keyARN {
		t.Errorf("GPI KeyId = %q, want %q", got, keyARN)
	}

	// ImportKeyMaterial reuses the token from the same GPI response and sets
	// the default no-expiry model.
	if !bytes.Equal(m.ikmInput.ImportToken, token) {
		t.Errorf("IKM ImportToken = %x, want %x", m.ikmInput.ImportToken, token)
	}
	if got := m.ikmInput.ExpirationModel; got != types.ExpirationModelTypeKeyMaterialDoesNotExpire {
		t.Errorf("IKM ExpirationModel = %q, want KEY_MATERIAL_DOES_NOT_EXPIRE", got)
	}
	if m.ikmInput.ValidTo != nil {
		t.Errorf("IKM ValidTo = %v, want nil", m.ikmInput.ValidTo)
	}

	// The encrypted material decrypts back to our key material under the GPI
	// public key, proving the wrapping used the correct wrapping key.
	rsaPart := m.ikmInput.EncryptedKeyMaterial[:priv.Size()]
	aesPart := m.ikmInput.EncryptedKeyMaterial[priv.Size():]
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, rsaPart, nil)
	if err != nil {
		t.Fatalf("decrypt wrapped AES key from IKM input: %v", err)
	}
	wantAES, err := aesKeyWrapPad(aesKey, material)
	if err != nil {
		t.Fatalf("re-wrap material: %v", err)
	}
	if !bytes.Equal(aesPart, wantAES) {
		t.Errorf("IKM EncryptedKeyMaterial does not round-trip to the key material")
	}
}

func TestImport_WithExpiry(t *testing.T) {
	_, pubDER := newWrappingKey(t)
	keyARN := "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"
	validTo := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)

	m := &mockKMS{
		gpiOutput: &kms.GetParametersForImportOutput{
			KeyId:       new(keyARN),
			PublicKey:   pubDER,
			ImportToken: []byte("token"),
		},
		ikmOutput: &kms.ImportKeyMaterialOutput{KeyId: new(keyARN)},
	}

	_, err := Import(context.Background(),
		WithClient(m),
		WithKeyID(keyARN),
		WithKeyMaterial(mustHex(t, "3082010203040506")),
		WithExpiry(validTo),
	)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if got := m.ikmInput.ExpirationModel; got != types.ExpirationModelTypeKeyMaterialExpires {
		t.Errorf("IKM ExpirationModel = %q, want KEY_MATERIAL_EXPIRES", got)
	}
	if got := aws.ToTime(m.ikmInput.ValidTo); !got.Equal(validTo) {
		t.Errorf("IKM ValidTo = %v, want %v", got, validTo)
	}
}

func TestImport_NoClient(t *testing.T) {
	_, err := Import(context.Background(),
		WithKeyID("alias/app"),
		WithKeyMaterial([]byte{1, 2, 3}),
	)
	if !errors.Is(err, ErrNoClient) {
		t.Fatalf("Import error = %v, want ErrNoClient", err)
	}
}

func TestImport_NoKeyMaterial(t *testing.T) {
	_, err := Import(context.Background(),
		WithClient(&mockKMS{}),
		WithKeyID("alias/app"),
	)
	if !errors.Is(err, ErrNoKeyMaterial) {
		t.Fatalf("Import error = %v, want ErrNoKeyMaterial", err)
	}
}

func TestImport_NoKeyID(t *testing.T) {
	_, err := Import(context.Background(),
		WithClient(&mockKMS{}),
		WithKeyMaterial([]byte{1, 2, 3}),
	)
	if !errors.Is(err, ErrNoKeyID) {
		t.Fatalf("Import error = %v, want ErrNoKeyID", err)
	}
}

func TestImport_GPIFailureSurfaces(t *testing.T) {
	sentinel := errors.New("gpi boom")
	m := &mockKMS{gpiErr: sentinel}

	_, err := Import(context.Background(),
		WithClient(m),
		WithKeyID("alias/app"),
		WithKeyMaterial([]byte{1, 2, 3}),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Import error = %v, want it to wrap %v", err, sentinel)
	}
	if m.ikmInput != nil {
		t.Errorf("ImportKeyMaterial was called despite GPI failure")
	}
}

func TestImport_IKMFailureSurfaces(t *testing.T) {
	_, pubDER := newWrappingKey(t)
	// Simulates KMS rejecting the call, e.g. an InvalidImportTokenException when
	// the token and wrapping public key do not correspond.
	sentinel := errors.New("InvalidImportTokenException")
	m := &mockKMS{
		gpiOutput: &kms.GetParametersForImportOutput{
			KeyId:       new("alias/app"),
			PublicKey:   pubDER,
			ImportToken: []byte("token"),
		},
		ikmErr: sentinel,
	}

	_, err := Import(context.Background(),
		WithClient(m),
		WithKeyID("alias/app"),
		WithKeyMaterial(mustHex(t, "3082010203040506")),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Import error = %v, want it to wrap %v", err, sentinel)
	}
}

func TestImport_NonRSAWrappingKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&ecKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal EC public key: %v", err)
	}
	m := &mockKMS{
		gpiOutput: &kms.GetParametersForImportOutput{
			KeyId:       new("alias/app"),
			PublicKey:   pubDER,
			ImportToken: []byte("token"),
		},
	}

	_, err = Import(context.Background(),
		WithClient(m),
		WithKeyID("alias/app"),
		WithKeyMaterial(mustHex(t, "3082010203040506")),
	)
	if err == nil {
		t.Fatal("Import succeeded with a non-RSA wrapping key, want error")
	}
	if m.ikmInput != nil {
		t.Errorf("ImportKeyMaterial was called with a non-RSA wrapping key")
	}
}

// The AWS SDK *kms.Client must satisfy the library's KMSClient interface so the
// caller can inject it directly. This is a compile-time assertion.
var _ KMSClient = (*kms.Client)(nil)

func TestImport_NilGPIResponse(t *testing.T) {
	// A misbehaving client returning (nil, nil) must surface an error, not panic.
	m := &mockKMS{gpiOutput: nil}

	_, err := Import(context.Background(),
		WithClient(m),
		WithKeyID("alias/app"),
		WithKeyMaterial(mustHex(t, "3082010203040506")),
	)
	if err == nil {
		t.Fatal("Import succeeded with a nil GPI response, want error")
	}
}

func TestImport_NilIKMResponse(t *testing.T) {
	_, pubDER := newWrappingKey(t)
	// GPI succeeds, but ImportKeyMaterial returns (nil, nil).
	m := &mockKMS{
		gpiOutput: &kms.GetParametersForImportOutput{
			KeyId:       new("alias/app"),
			PublicKey:   pubDER,
			ImportToken: []byte("token"),
		},
		ikmOutput: nil,
	}

	_, err := Import(context.Background(),
		WithClient(m),
		WithKeyID("alias/app"),
		WithKeyMaterial(mustHex(t, "3082010203040506")),
	)
	if err == nil {
		t.Fatal("Import succeeded with a nil IKM response, want error")
	}
}
