package buildinfo_test

import (
	"bytes"
	"testing"

	"github.com/chinmina/kms-import/internal/buildinfo"
)

func TestFprint_WritesNameAndVersion(t *testing.T) {
	var buf bytes.Buffer

	if err := buildinfo.Fprint(&buf, "kms-import"); err != nil {
		t.Fatalf("Fprint returned error: %v", err)
	}

	if got, want := buf.String(), "kms-import dev\n"; got != want {
		t.Errorf("Fprint wrote %q, want %q", got, want)
	}
}
