package kmsimport

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// KeyMaterialFromPEM decodes a PEM-encoded RSA private key and returns it as the
// PKCS#8 DER that Import (via WithKeyMaterial) expects. Decoding the operator's
// key file is part of pushing a key, so it lives in the library alongside the
// wrapping crypto rather than in any CLI wrapper.
//
// The format is detected from the PEM block Type: GitHub issues App keys in
// PKCS#1 ("BEGIN RSA PRIVATE KEY"), which is converted to PKCS#8 DER; PKCS#8
// ("BEGIN PRIVATE KEY") is accepted as-is. Any other header is rejected with an
// error naming the unsupported format.
func KeyMaterialFromPEM(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("decode PEM: no PEM block found")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1 private key: %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("marshal PKCS#8 private key: %w", err)
		}
		return der, nil
	case "PRIVATE KEY":
		// Already PKCS#8; validate it parses and is an RSA key (the only key
		// type this tool supports) before returning the DER as-is.
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
		}
		if _, ok := key.(*rsa.PrivateKey); !ok {
			return nil, fmt.Errorf("unsupported private key type %T: only RSA keys are supported", key)
		}
		return block.Bytes, nil
	default:
		return nil, fmt.Errorf("unsupported PEM format %q: want \"RSA PRIVATE KEY\" (PKCS#1) or \"PRIVATE KEY\" (PKCS#8)", block.Type)
	}
}
