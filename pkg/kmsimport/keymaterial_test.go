package kmsimport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

// pkcs1PEM returns a PKCS#1 ("BEGIN RSA PRIVATE KEY") PEM encoding of priv,
// which is GitHub's private key output format.
func pkcs1PEM(t *testing.T, priv *rsa.PrivateKey) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
}

// pkcs8PEM returns a PKCS#8 ("BEGIN PRIVATE KEY") PEM encoding of priv, the
// convenience format accepted alongside GitHub's PKCS#1 output.
func pkcs8PEM(t *testing.T, priv *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})
}

func TestKeyMaterialFromPEM_PKCS8(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	der, err := KeyMaterialFromPEM(pkcs8PEM(t, priv))
	if err != nil {
		t.Fatalf("KeyMaterialFromPEM returned error: %v", err)
	}

	got, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		t.Fatalf("result is not PKCS#8 DER: %v", err)
	}
	gotRSA, ok := got.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("result key type = %T, want *rsa.PrivateKey", got)
	}
	if !gotRSA.Equal(priv) {
		t.Errorf("decoded key does not equal the original PKCS#8 key")
	}
}

func TestKeyMaterialFromPEM_UnsupportedHeader(t *testing.T) {
	unsupported := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: []byte("irrelevant"),
	})

	_, err := KeyMaterialFromPEM(unsupported)
	if err == nil {
		t.Fatal("KeyMaterialFromPEM succeeded on unsupported header, want error")
	}
	if !strings.Contains(err.Error(), "EC PRIVATE KEY") {
		t.Errorf("error %q does not name the unsupported format %q", err, "EC PRIVATE KEY")
	}
}

func TestKeyMaterialFromPEM_PKCS8NonRSA(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	ecPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err = KeyMaterialFromPEM(ecPEM)
	if err == nil {
		t.Fatal("KeyMaterialFromPEM accepted a non-RSA PKCS#8 key, want error")
	}
	if !strings.Contains(err.Error(), "RSA") {
		t.Errorf("error %q does not explain that only RSA keys are supported", err)
	}
}

func TestKeyMaterialFromPEM_Undecodable(t *testing.T) {
	_, err := KeyMaterialFromPEM([]byte("this is not a PEM file"))
	if err == nil {
		t.Fatal("KeyMaterialFromPEM succeeded on undecodable input, want error")
	}
}

func TestKeyMaterialFromPEM_PKCS1(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	der, err := KeyMaterialFromPEM(pkcs1PEM(t, priv))
	if err != nil {
		t.Fatalf("KeyMaterialFromPEM returned error: %v", err)
	}

	// The result must be PKCS#8 DER that round-trips back to the same key.
	got, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		t.Fatalf("result is not PKCS#8 DER: %v", err)
	}
	gotRSA, ok := got.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("result key type = %T, want *rsa.PrivateKey", got)
	}
	if !gotRSA.Equal(priv) {
		t.Errorf("decoded key does not equal the original PKCS#1 key")
	}
}
