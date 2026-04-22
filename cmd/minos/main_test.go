package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puria/minos/internal/config"
)

func TestNewRootCmdUsesPositionalRoot(t *testing.T) {
	root := t.TempDir()
	var got config.Config
	cmd := newRootCmd(config.Default(), "v1.2.3", func(cfg config.Config, _ string) error {
		got = cfg
		return nil
	})
	cmd.SetArgs([]string{root})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Root != root {
		t.Fatalf("expected root %q, got %q", root, got.Root)
	}
}

func TestNewRootCmdPrefersFlagRoot(t *testing.T) {
	positional := t.TempDir()
	flagRoot := t.TempDir()
	var got config.Config
	cmd := newRootCmd(config.Default(), "v1.2.3", func(cfg config.Config, _ string) error {
		got = cfg
		return nil
	})
	cmd.SetArgs([]string{"--root", flagRoot, positional})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Root != flagRoot {
		t.Fatalf("expected flag root %q, got %q", flagRoot, got.Root)
	}
}

func TestExecutePassesVersionAndFlags(t *testing.T) {
	previous := version
	version = "v9.9.9"
	t.Cleanup(func() { version = previous })

	root := t.TempDir()
	var gotCfg config.Config
	var gotVersion string
	err := execute([]string{
		"--root", root,
		"--show-clean-only",
		"--show-safe-only",
		"--safe-remove-requires-no-extra-branches=false",
		"--safe-remove-requires-no-linked-worktrees=false",
		"--debug",
	}, func(cfg config.Config, v string) error {
		gotCfg = cfg
		gotVersion = v
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotVersion != "v9.9.9" {
		t.Fatalf("unexpected version: %q", gotVersion)
	}
	if !gotCfg.ShowCleanOnly || !gotCfg.ShowSafeOnly || !gotCfg.Debug {
		t.Fatalf("expected flags to propagate: %#v", gotCfg)
	}
	if gotCfg.SafeRemoveRequiresNoExtraBranches || gotCfg.SafeRemoveRequiresNoLinkedWorktrees {
		t.Fatalf("expected safe-remove overrides to propagate: %#v", gotCfg)
	}
}

func TestExecuteReturnsNormalizeError(t *testing.T) {
	err := execute([]string{"--root", filepath.Join(t.TempDir(), "missing")}, func(cfg config.Config, v string) error {
		t.Fatalf("run should not be called")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "stat root") {
		t.Fatalf("expected normalize error, got %v", err)
	}
}

func TestExecuteReturnsRunError(t *testing.T) {
	want := errors.New("boom")
	err := execute([]string{t.TempDir()}, func(cfg config.Config, v string) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected run error, got %v", err)
	}
}
