package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

func TestGenerateKey_PKCS1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := generateKey("pkcs1", path); err != nil {
		t.Fatalf("generateKey(pkcs1): %v", err)
	}

	block := readPEM(t, path)
	if block.Type != "RSA PRIVATE KEY" {
		t.Errorf("PEM type = %q, want RSA PRIVATE KEY", block.Type)
	}
	assertRSA2048(t, block.Bytes)
	assertFileMode(t, path, 0o600)
}

func TestGenerateKey_PKCS8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := generateKey("pkcs8", path); err != nil {
		t.Fatalf("generateKey(pkcs8): %v", err)
	}

	block := readPEM(t, path)
	if block.Type != "PRIVATE KEY" {
		t.Errorf("PEM type = %q, want PRIVATE KEY", block.Type)
	}
	assertRSA2048(t, block.Bytes)
	assertFileMode(t, path, 0o600)
}

func TestGenerateKey_RefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.pem")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	err := generateKey("pkcs8", path)
	if err == nil {
		t.Fatal("generateKey over existing file succeeded, want error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q does not mention already exists", err.Error())
	}
}

func TestGenerateKey_DirectoryPathIsNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "key.pem")
	err := generateKey("pkcs8", path)
	if err == nil {
		t.Fatal("generateKey into missing directory succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %q is not fs.ErrNotExist", err)
	}
}

func TestGenerateKeyCommand_PKCS1Flag(t *testing.T) {
	cmd := command()
	var out bytes.Buffer
	cmd.Writer = &out

	path := filepath.Join(t.TempDir(), "key.pem")
	err := cmd.Run(context.Background(), []string{
		"kms-support", "generate-key",
		"--pkcs1", "--output", path,
	})
	if err != nil {
		t.Fatalf("generate-key --pkcs1: %v", err)
	}

	block := readPEM(t, path)
	if block.Type != "RSA PRIVATE KEY" {
		t.Errorf("PEM type = %q, want RSA PRIVATE KEY", block.Type)
	}
}

func TestGenerateKeyCommand_NoFormatFlag_Errors(t *testing.T) {
	cmd := command()
	var out bytes.Buffer
	cmd.Writer = &out

	err := cmd.Run(context.Background(), []string{
		"kms-support", "generate-key",
		"--output", filepath.Join(t.TempDir(), "key.pem"),
	})
	if err == nil {
		t.Fatal("generate-key without format flag succeeded, want error")
	}
	for _, flag := range []string{"pkcs1", "pkcs8"} {
		if !strings.Contains(err.Error(), flag) {
			t.Errorf("error %q does not mention %q", err.Error(), flag)
		}
	}
}

func TestGenerateKeyCommand_BothFormatFlags_Errors(t *testing.T) {
	cmd := command()
	var out bytes.Buffer
	cmd.Writer = &out

	err := cmd.Run(context.Background(), []string{
		"kms-support", "generate-key",
		"--pkcs1", "--pkcs8",
		"--output", filepath.Join(t.TempDir(), "key.pem"),
	})
	if err == nil {
		t.Fatal("generate-key with both format flags succeeded, want error")
	}
	if !strings.Contains(err.Error(), "cannot be set along with") {
		t.Errorf("error %q should indicate flags cannot be combined", err.Error())
	}
}

func TestCreateTargetKey_Success(t *testing.T) {
	fake := &fakeCreateKeyClient{keyID: "1234abcd-12ab-34cd-56ef-1234567890ab"}
	var out bytes.Buffer

	err := createTargetKey(context.Background(), fake, &out)
	if err != nil {
		t.Fatalf("createTargetKey: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != fake.keyID {
		t.Errorf("stdout = %q, want %q", got, fake.keyID)
	}

	if fake.input == nil {
		t.Fatal("CreateKey was not called")
	}
	if fake.input.Origin != types.OriginTypeExternal {
		t.Errorf("Origin = %q, want EXTERNAL", fake.input.Origin)
	}
	if fake.input.KeySpec != types.KeySpecRsa2048 {
		t.Errorf("KeySpec = %q, want RSA_2048", fake.input.KeySpec)
	}
	if fake.input.KeyUsage != types.KeyUsageTypeSignVerify {
		t.Errorf("KeyUsage = %q, want SIGN_VERIFY", fake.input.KeyUsage)
	}
}

func TestCreateTargetKey_NilClient(t *testing.T) {
	err := createTargetKey(context.Background(), nil, &bytes.Buffer{})
	if err == nil {
		t.Fatal("createTargetKey with nil client succeeded, want error")
	}
}

func TestCreateTargetKey_ClientError(t *testing.T) {
	sentinel := errors.New("network error")
	fake := &fakeCreateKeyClient{err: sentinel}

	err := createTargetKey(context.Background(), fake, &bytes.Buffer{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("createTargetKey error = %v, want %v", err, sentinel)
	}
}

func TestCreateTargetKey_MissingKeyID(t *testing.T) {
	fake := &fakeCreateKeyClient{}
	var out bytes.Buffer

	err := createTargetKey(context.Background(), fake, &out)
	if err == nil {
		t.Fatal("createTargetKey with empty response succeeded, want error")
	}
}

func TestCreateKeyCommand_UsesEnvironment(t *testing.T) {
	// The command itself has no flags; it relies on the AWS SDK chain. This
	// test proves the command validates and can be invoked with no extra
	// arguments (it will fail to load config without AWS region/credentials).
	cmd := command()
	cmd.Writer = &bytes.Buffer{}

	err := cmd.Run(context.Background(), []string{"kms-support", "kms-create-key"})
	if err == nil {
		t.Fatal("kms-create-key without AWS config succeeded, want error")
	}
}

func readPEM(t *testing.T, path string) *pem.Block {
	t.Helper()
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated PEM: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatalf("no PEM block found in %q", path)
	}
	return block
}

func assertRSA2048(t *testing.T, der []byte) {
	t.Helper()
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		var pkcs1Err error
		key, pkcs1Err = x509.ParsePKCS1PrivateKey(der)
		if pkcs1Err != nil {
			t.Fatalf("parse generated private key: %v", err)
		}
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("generated key is %T, want *rsa.PrivateKey", key)
	}
	if rsaKey.N.BitLen() != rsaKeyBits {
		t.Errorf("key size = %d bits, want %d", rsaKey.N.BitLen(), rsaKeyBits)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("file mode = 0o%o, want 0o%o", got, want)
	}
}

type fakeCreateKeyClient struct {
	input *kms.CreateKeyInput
	keyID string
	err   error
}

func (f *fakeCreateKeyClient) CreateKey(_ context.Context, in *kms.CreateKeyInput, _ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
	f.input = in
	if f.err != nil {
		return nil, f.err
	}
	return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{KeyId: aws.String(f.keyID)}}, nil
}
