package kmsimport

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// mustHex decodes a hex string for test fixtures, failing the test on error.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode hex %q: %v", s, err)
	}
	return b
}

// TestAESKeyWrapPad_RFC5649MultiBlock checks aesKeyWrapPad against the
// multi-block known-answer test vector from RFC 5649 section 6 (20 octets of
// key data wrapped with a 192-bit KEK). This is the general padded-key-wrap
// path used for real PKCS#8 key material.
func TestAESKeyWrapPad_RFC5649MultiBlock(t *testing.T) {
	kek := mustHex(t, "5840df6e29b02af1ab493b705bf16ea1ae8338f4dcc176a8")
	plaintext := mustHex(t, "c37b7e6492584340bed12207808941155068f738")
	want := mustHex(t, "138bdeaa9b8fa7fc61f97742e72248ee5ae6ae5360d1ae6a5f54f373fa543b6a")

	got, err := aesKeyWrapPad(kek, plaintext)
	if err != nil {
		t.Fatalf("aesKeyWrapPad returned error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("aesKeyWrapPad =\n  %x\nwant\n  %x", got, want)
	}
}

// TestAESKeyWrapPad_RFC5649SingleBlock checks the single-block path from RFC
// 5649 section 6 (7 octets padded to a single 8-octet block). RFC 5649 requires
// this case to be a single AES-ECB encryption of AIV||paddedplaintext rather
// than the iterative integrity loop.
func TestAESKeyWrapPad_RFC5649SingleBlock(t *testing.T) {
	kek := mustHex(t, "5840df6e29b02af1ab493b705bf16ea1ae8338f4dcc176a8")
	plaintext := mustHex(t, "466f7250617369")
	want := mustHex(t, "afbeb0f07dfbf5419200f2ccb50bb24f")

	got, err := aesKeyWrapPad(kek, plaintext)
	if err != nil {
		t.Fatalf("aesKeyWrapPad returned error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("aesKeyWrapPad =\n  %x\nwant\n  %x", got, want)
	}
}

// TestWrapKeyMaterial_RSAAESKeyWrapSHA256 verifies the full
// RSA_AES_KEY_WRAP_SHA_256 wire format: the output is
// (RSA-OAEP-SHA256-encrypted AES key) || (RFC 5649 AES-wrapped key material).
// It recovers the ephemeral AES key via OAEP decryption and confirms the
// trailing bytes match the deterministic AES wrap of the material under it.
func TestWrapKeyMaterial_RSAAESKeyWrapSHA256(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate wrapping key: %v", err)
	}
	material := mustHex(t, "3082010203040506") // stand-in PKCS#8 DER bytes

	out, err := wrapKeyMaterial(&priv.PublicKey, material)
	if err != nil {
		t.Fatalf("wrapKeyMaterial returned error: %v", err)
	}

	rsaPart, aesPart := out[:priv.Size()], out[priv.Size():]

	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, rsaPart, nil)
	if err != nil {
		t.Fatalf("OAEP-decrypt wrapped AES key: %v", err)
	}
	if len(aesKey) != 32 {
		t.Fatalf("recovered AES key is %d bytes, want 32 (AES-256)", len(aesKey))
	}

	wantAES, err := aesKeyWrapPad(aesKey, material)
	if err != nil {
		t.Fatalf("re-wrap material with recovered key: %v", err)
	}
	if !bytes.Equal(aesPart, wantAES) {
		t.Errorf("AES-wrapped material =\n  %x\nwant\n  %x", aesPart, wantAES)
	}
}
