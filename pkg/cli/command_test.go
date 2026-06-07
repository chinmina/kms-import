package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	clipkg "github.com/urfave/cli/v3"
)

// Command must return a mountable urfave/cli v3 *cli.Command (R33).
var _ func() *clipkg.Command = Command

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
