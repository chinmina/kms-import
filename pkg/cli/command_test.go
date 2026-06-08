package cli

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

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

func TestCommand_HelpListsFlags(t *testing.T) {
	cmd := Command()

	var out bytes.Buffer
	cmd.Writer = &out
	if err := cmd.Run(context.Background(), []string{"kms-import", "--help"}); err != nil {
		t.Fatalf("run --help: %v", err)
	}

	help := out.String()
	for _, want := range []string{"--key-file", "--key-id"} {
		if !strings.Contains(help, want) {
			t.Errorf("help output does not mention %q\n%s", want, help)
		}
	}
}
