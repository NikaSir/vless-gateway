package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCLIRequiresExplicitCommandsAndRootForMutation(t *testing.T) {
	for _, args := range [][]string{{}, {"bogus"}, {"set-mode", "blackhole"}, {"restore", "--backup", "/tmp/a"}, {"reload"}, {"rules-update"}} {
		var out bytes.Buffer
		e := RunCLI(context.Background(), args, &out, 1000)
		if e == nil {
			t.Fatalf("invalid/non-root command accepted: %v", args)
		}
	}
	var out bytes.Buffer
	if e := RunCLI(context.Background(), []string{"--version"}, &out, 1000); e != nil || !strings.Contains(out.String(), "schema") {
		t.Fatal(out.String(), e)
	}
}
