// Package kmsimport imports asymmetric RSA private key material into an AWS KMS
// key created with EXTERNAL origin, using the RSA_AES_KEY_WRAP_SHA_256 wrapping
// algorithm.
//
// The package is a library first: Import takes a caller-supplied KMSClient and
// PKCS#8 DER key material, performs the GetParametersForImport / wrap /
// ImportKeyMaterial sequence, and returns a structured Result and an error. It
// never constructs an AWS SDK client and never calls os.Exit.
package kmsimport

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// ErrNoClient is returned by Import when no KMSClient was provided.
var ErrNoClient = errors.New("kmsimport: no KMS client provided")

// ErrNoKeyMaterial is returned by Import when no key material was provided.
var ErrNoKeyMaterial = errors.New("kmsimport: no key material provided")

// ErrNoKeyID is returned by Import when no key identifier was provided.
var ErrNoKeyID = errors.New("kmsimport: no key identifier provided")

// KMSClient is the subset of the AWS KMS API that Import requires. The AWS SDK
// *kms.Client satisfies this interface directly. The library never constructs a
// client; the caller injects one.
type KMSClient interface {
	GetParametersForImport(ctx context.Context, params *kms.GetParametersForImportInput, optFns ...func(*kms.Options)) (*kms.GetParametersForImportOutput, error)
	ImportKeyMaterial(ctx context.Context, params *kms.ImportKeyMaterialInput, optFns ...func(*kms.Options)) (*kms.ImportKeyMaterialOutput, error)
}

// Result describes the outcome of a successful import.
type Result struct {
	// KeyID is the KMS key identifier resolved by the import operation.
	KeyID string
	// KeyState is the key state following a successful import. It is always
	// "Enabled": a successful ImportKeyMaterial transitions the key out of
	// PendingImport, and the two-method KMSClient interface deliberately omits
	// DescribeKey, so the state is inferred rather than queried.
	KeyState string
}

// Option configures an Import call.
type Option func(*options)

type options struct {
	client      KMSClient
	keyID       string
	keyMaterial []byte
	expiry      time.Time
}

// WithClient injects the KMS client used to perform the import.
func WithClient(c KMSClient) Option {
	return func(o *options) { o.client = c }
}

// WithKeyID sets the target KMS key identifier, passed verbatim to the import
// APIs. KMS accepts a key ID or key ARN here (the import operations do not
// resolve aliases).
func WithKeyID(id string) Option {
	return func(o *options) { o.keyID = id }
}

// WithKeyMaterial sets the private key material to import, as PKCS#8 DER.
func WithKeyMaterial(der []byte) Option {
	return func(o *options) { o.keyMaterial = der }
}

// WithExpiry sets an expiry time for the imported key material. When provided,
// the import uses ExpirationModel KEY_MATERIAL_EXPIRES with ValidTo set to t.
// Absent this option the material does not expire.
func WithExpiry(t time.Time) Option {
	return func(o *options) { o.expiry = t }
}

// Import imports key material into the target KMS key. It fetches the wrapping
// public key and import token via GetParametersForImport, encrypts the key
// material per RSA_AES_KEY_WRAP_SHA_256, and submits it with the matching import
// token via ImportKeyMaterial. By default the imported material does not expire.
func Import(ctx context.Context, opts ...Option) (Result, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if o.client == nil {
		return Result{}, ErrNoClient
	}
	if o.keyID == "" {
		return Result{}, ErrNoKeyID
	}
	if len(o.keyMaterial) == 0 {
		return Result{}, ErrNoKeyMaterial
	}

	params, err := o.client.GetParametersForImport(ctx, &kms.GetParametersForImportInput{
		KeyId:             new(o.keyID),
		WrappingAlgorithm: types.AlgorithmSpecRsaAesKeyWrapSha256,
		WrappingKeySpec:   types.WrappingKeySpecRsa4096,
	})
	if err != nil {
		return Result{}, fmt.Errorf("get parameters for import: %w", err)
	}
	if params == nil {
		return Result{}, errors.New("get parameters for import: nil response")
	}

	wrappingKey, err := parseWrappingKey(params.PublicKey)
	if err != nil {
		return Result{}, err
	}

	encrypted, err := wrapKeyMaterial(wrappingKey, o.keyMaterial)
	if err != nil {
		return Result{}, fmt.Errorf("wrap key material: %w", err)
	}

	ikmInput := &kms.ImportKeyMaterialInput{
		KeyId:                new(o.keyID),
		ImportToken:          params.ImportToken,
		EncryptedKeyMaterial: encrypted,
		ExpirationModel:      types.ExpirationModelTypeKeyMaterialDoesNotExpire,
	}
	if !o.expiry.IsZero() {
		ikmInput.ExpirationModel = types.ExpirationModelTypeKeyMaterialExpires
		ikmInput.ValidTo = aws.Time(o.expiry)
	}

	out, err := o.client.ImportKeyMaterial(ctx, ikmInput)
	if err != nil {
		return Result{}, fmt.Errorf("import key material: %w", err)
	}
	if out == nil {
		return Result{}, errors.New("import key material: nil response")
	}

	keyID := o.keyID
	if id := aws.ToString(out.KeyId); id != "" {
		keyID = id
	}

	// A successful ImportKeyMaterial leaves the key Enabled. We report that
	// directly rather than confirming via DescribeKey, which is outside the
	// two-method KMSClient interface this library commits to.
	return Result{
		KeyID:    keyID,
		KeyState: string(types.KeyStateEnabled),
	}, nil
}

// parseWrappingKey decodes the SubjectPublicKeyInfo DER returned by
// GetParametersForImport into an RSA public key.
func parseWrappingKey(der []byte) (*rsa.PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse wrapping public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("wrapping public key is not an RSA key")
	}
	return rsaPub, nil
}
