package main

import (
	"bytes"
	"context"
	"testing"
)

func TestUnlockFilesNothingLocked(t *testing.T) {
	var out bytes.Buffer
	if err := unlockFiles(context.Background(), t.TempDir(), nil, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "No website's files are locked.\n" {
		t.Fatalf("%q", out.String())
	}
}
