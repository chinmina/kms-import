package main

import (
	"io"
	"testing"
)

// With no flags supplied, the mounted command must fail because --key-file is
// required. This proves cmd/ delegates to the pkg/cli Command rather than the
// old tracer-bullet usage path.
func TestRun_RequiresKeyFile(t *testing.T) {
	err := run([]string{"kms-import"}, io.Discard)
	if err == nil {
		t.Fatal("run with no flags returned nil, want a required-flag error")
	}
}
