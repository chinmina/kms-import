package kmsimport

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
)

// wrapKeyMaterial encrypts PKCS#8 DER key material for import into KMS using the
// RSA_AES_KEY_WRAP_SHA_256 wrapping algorithm and the wrapping public key
// returned by GetParametersForImport. It generates an ephemeral AES-256 key,
// wraps the key material with it (RFC 5649), encrypts that AES key under pub
// with RSA-OAEP-SHA-256, and returns the concatenation
// (encrypted AES key || AES-wrapped key material) that ImportKeyMaterial expects.
func wrapKeyMaterial(pub *rsa.PublicKey, keyMaterial []byte) ([]byte, error) {
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("generate ephemeral AES key: %w", err)
	}

	wrappedMaterial, err := aesKeyWrapPad(aesKey, keyMaterial)
	if err != nil {
		return nil, fmt.Errorf("AES key-wrap key material: %w", err)
	}

	wrappedAESKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA-OAEP encrypt ephemeral AES key: %w", err)
	}

	return append(wrappedAESKey, wrappedMaterial...), nil
}

// rfc5649AIV is the high-order half of the Alternative Initial Value defined by
// RFC 5649 (AES Key Wrap with Padding). The low-order half carries the message
// length indicator.
var rfc5649AIV = []byte{0xA6, 0x59, 0x59, 0xA6}

// aesKeyWrapPad wraps plaintext under the key-encryption key kek using the AES
// Key Wrap with Padding algorithm (RFC 5649), as required by the KMS
// RSA_AES_KEY_WRAP_SHA_256 wrapping algorithm. The kek must be a valid AES key
// length (16, 24, or 32 bytes); KMS uses 32 (AES-256).
func aesKeyWrapPad(kek, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}

	// RFC 5649 encodes the plaintext length in a 32-bit message length
	// indicator, so the input must be non-empty and fit in a uint32.
	if len(plaintext) == 0 || uint64(len(plaintext)) > math.MaxUint32 {
		return nil, fmt.Errorf("aes key-wrap: plaintext length %d out of range", len(plaintext))
	}

	// Build the 64-bit AIV: 0xA65959A6 || MLI, where MLI is the unpadded
	// plaintext length in octets, big-endian.
	aiv := make([]byte, 8)
	copy(aiv, rfc5649AIV)
	//nolint:gosec // length is bounded above by the MaxUint32 check above.
	binary.BigEndian.PutUint32(aiv[4:], uint32(len(plaintext)))

	// Zero-pad the plaintext up to an 8-octet boundary.
	padded := make([]byte, ((len(plaintext)+7)/8)*8)
	copy(padded, plaintext)

	// RFC 5649: when the padded plaintext is a single 64-bit block, the wrap
	// is one AES-ECB encryption of AIV||padded; otherwise run the RFC 3394
	// integrity loop with AIV as the initial register.
	if len(padded) == 8 {
		out := make([]byte, 16)
		in := make([]byte, 16)
		copy(in, aiv)
		copy(in[8:], padded)
		block.Encrypt(out, in)
		return out, nil
	}

	return aesWrapCore(block, aiv, padded), nil
}

// aesWrapCore performs the RFC 3394 key-wrap integrity loop over the padded
// plaintext using iv as the initial integrity register. padded must be a
// non-empty multiple of 8 octets.
func aesWrapCore(block cipher.Block, iv, padded []byte) []byte {
	n := len(padded) / 8

	a := make([]byte, 8)
	copy(a, iv)
	r := make([]byte, len(padded))
	copy(r, padded)

	in := make([]byte, 16)
	out := make([]byte, 16)
	var tb [8]byte

	for j := range 6 {
		for i := 1; i <= n; i++ {
			copy(in[:8], a)
			copy(in[8:], r[(i-1)*8:i*8])
			block.Encrypt(out, in)

			copy(a, out[:8])
			binary.BigEndian.PutUint64(tb[:], uint64(n*j+i))
			for k := range 8 {
				a[k] ^= tb[k]
			}
			copy(r[(i-1)*8:i*8], out[8:])
		}
	}

	wrapped := make([]byte, 8+len(r))
	copy(wrapped, a)
	copy(wrapped[8:], r)
	return wrapped
}
