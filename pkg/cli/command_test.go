package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clipkg "github.com/urfave/cli/v3"
)

// validPEM generates a minimal PKCS#1 RSA-2048 PEM for CLI-level tests.
func validPEM(t *testing.T) []byte {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
}

// Command must return a mountable urfave/cli v3 *cli.Command (R33).
var _ func() *clipkg.Command = Command

func TestCommand_UnreadableKeyFile(t *testing.T) {
	cmd := Command()

	missing := filepath.Join(t.TempDir(), "does-not-exist.pem")
	err := cmd.Run(context.Background(), []string{
		"kms-import",
		"--key-file", missing,
		"--key-id", "test-key",
	})
	if err == nil {
		t.Fatal("run with unreadable key file succeeded, want error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not identify the file %q", err, missing)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %q does not wrap fs.ErrNotExist", err)
	}
}

// TestCommand_NoTargetFlag_Errors checks that omitting all target flags produces
// an error that names all three options (R9).
func TestCommand_NoTargetFlag_Errors(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, validPEM(t), 0600); err != nil {
		t.Fatalf("write temp PEM: %v", err)
	}

	err := Command().Run(context.Background(), []string{"kms-import", "--key-file", keyFile})
	if err == nil {
		t.Fatal("want error when no target flag provided, got nil")
	}
	for _, flag := range []string{"--key-id", "--key-arn", "--alias"} {
		if !strings.Contains(err.Error(), flag) {
			t.Errorf("error %q does not mention %q", err.Error(), flag)
		}
	}
}

// TestCommand_TwoTargetFlags_Errors checks that providing more than one target
// flag is rejected (R8).
func TestCommand_TwoTargetFlags_Errors(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, validPEM(t), 0600); err != nil {
		t.Fatalf("write temp PEM: %v", err)
	}

	err := Command().Run(context.Background(), []string{
		"kms-import",
		"--key-file", keyFile,
		"--key-id", "abcd-1234",
		"--key-arn", "arn:aws:kms:us-east-1:111122223333:key/abcd-1234",
	})
	if err == nil {
		t.Fatal("want error when two target flags provided, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error %q should mention mutual exclusivity", err.Error())
	}
}

// TestNormaliseAlias checks that a bare alias name gets "alias/" prepended (R10)
// and that an already-prefixed alias is used as-is (R11).
func TestNormaliseAlias(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"my-app-key", "alias/my-app-key"},
		{"alias/my-app-key", "alias/my-app-key"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := normaliseAlias(tc.input)
			if got != tc.want {
				t.Errorf("normaliseAlias(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCommand_HelpListsFlags(t *testing.T) {
	cmd := Command()

	var out bytes.Buffer
	cmd.Writer = &out
	if err := cmd.Run(context.Background(), []string{"kms-import", "--help"}); err != nil {
		t.Fatalf("run --help: %v", err)
	}

	help := out.String()
	for _, want := range []string{"--key-file", "--key-id", "--key-arn", "--alias", "--profile", "--region"} {
		if !strings.Contains(help, want) {
			t.Errorf("help output does not mention %q\n%s", want, help)
		}
	}
}
