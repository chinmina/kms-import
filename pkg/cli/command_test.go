package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clipkg "github.com/urfave/cli/v3"
)

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
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, pkcs1PEM(t, priv), 0600); err != nil {
		t.Fatalf("write temp PEM: %v", err)
	}

	err = Command().Run(context.Background(), []string{"kms-import", "--key-file", keyFile})
	if err == nil {
		t.Fatal("want error when no target flag provided, got nil")
	}
	for _, flag := range []string{"key-id", "key-arn"} {
		if !strings.Contains(err.Error(), flag) {
			t.Errorf("error %q does not mention %q", err.Error(), flag)
		}
	}
}

// TestCommand_TwoTargetFlags_Errors checks that providing more than one target
// flag is rejected (R8).
func TestCommand_TwoTargetFlags_Errors(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, pkcs1PEM(t, priv), 0600); err != nil {
		t.Fatalf("write temp PEM: %v", err)
	}

	err = Command().Run(context.Background(), []string{
		"kms-import",
		"--key-file", keyFile,
		"--key-id", "abcd-1234",
		"--key-arn", "arn:aws:kms:us-east-1:111122223333:key/abcd-1234",
	})
	if err == nil {
		t.Fatal("want error when two target flags provided, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be set along with") {
		t.Errorf("error %q should indicate flags cannot be combined", err.Error())
	}
}

// TestParseExpiry_Valid checks that a valid RFC 3339 future timestamp parses to
// the expected time (R17 expiry format).
func TestParseExpiry_Valid(t *testing.T) {
	got, err := parseExpiry("2099-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("parseExpiry returned error: %v", err)
	}
	want := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseExpiry = %v, want %v", got, want)
	}
}

// TestParseExpiry_PastRejected checks that an already-passed expiry date is
// rejected (R18).
func TestParseExpiry_PastRejected(t *testing.T) {
	_, err := parseExpiry("2000-01-01T00:00:00Z")
	if err == nil {
		t.Fatal("parseExpiry accepted a past date, want error")
	}
}

// TestCommand_PastExpiry_Errors checks that a past --expires date fails fast via
// the validation path (before any AWS call), not as an unknown flag or AWS error
// (R18).
func TestCommand_PastExpiry_Errors(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, pkcs1PEM(t, priv), 0600); err != nil {
		t.Fatalf("write temp PEM: %v", err)
	}

	err = Command().Run(context.Background(), []string{
		"kms-import",
		"--key-file", keyFile,
		"--key-id", "abcd-1234",
		"--expires", "2000-01-01T00:00:00Z",
	})
	if err == nil {
		t.Fatal("want error for past --expires, got nil")
	}
	if !strings.Contains(err.Error(), "past") {
		t.Errorf("error %q should report the expiry is in the past", err.Error())
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
	for _, want := range []string{"--key-file", "--key-id", "--key-arn", "--profile", "--region", "--expires", "--json"} {
		if !strings.Contains(help, want) {
			t.Errorf("help output does not mention %q\n%s", want, help)
		}
	}
}
