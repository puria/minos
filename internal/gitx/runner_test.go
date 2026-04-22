package gitx

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunnerRun(t *testing.T) {
	out, err := (ExecRunner{}).Run(context.Background(), t.TempDir(), "sh", "-c", "printf 'ok'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestExecRunnerRunIncludesStderrOnError(t *testing.T) {
	_, err := (ExecRunner{}).Run(context.Background(), t.TempDir(), "sh", "-c", "echo fail 1>&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Fatalf("expected stderr in error, got %v", err)
	}
}
