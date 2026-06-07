package cli

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// decodeKeyMaterial decodes a PEM-encoded RSA private key and returns it as
// PKCS#8 DER, the format the import library expects. GitHub issues App keys in
// PKCS#1 ("BEGIN RSA PRIVATE KEY") format, which is converted here.
func decodeKeyMaterial(pemBytes []byte) ([]byte, error) {
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
