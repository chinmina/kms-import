package kmsimport

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// KeyMaterialFromPEM decodes a PEM-encoded RSA private key and returns it as the
// PKCS#8 DER that Import (via WithKeyMaterial) expects. Decoding the operator's
// key file is part of pushing a key, so it lives in the library alongside the
// wrapping crypto rather than in any CLI wrapper.
//
// GitHub issues App keys in PKCS#1 ("BEGIN RSA PRIVATE KEY") format, which is
// converted here. Further format coverage (e.g. PKCS#8 "BEGIN PRIVATE KEY") is
// added in a later phase.
func KeyMaterialFromPEM(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("decode PEM: no PEM block found")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#1 private key: %w", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal PKCS#8 private key: %w", err)
	}
	return der, nil
}
