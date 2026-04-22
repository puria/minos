package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if !strings.HasSuffix(cfg.Root, string(filepath.Separator)+"src") {
		t.Fatalf("unexpected default root: %q", cfg.Root)
	}
	if cfg.SummarizerCmd == "" {
		t.Fatalf("expected summarizer command")
	}
	if !cfg.SafeRemoveRequiresNoExtraBranches || !cfg.SafeRemoveRequiresNoLinkedWorktrees {
		t.Fatalf("expected safe remove flags enabled by default")
	}
	if cfg.DiscoveryWorkers < 1 || cfg.GitWorkers < 1 {
		t.Fatalf("expected worker counts to be positive: %#v", cfg)
	}
}

func TestNormalizeSuccessAndClampWorkers(t *testing.T) {
	root := t.TempDir()
	cfg := Config{Root: root, DiscoveryWorkers: 0, GitWorkers: 0}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Root != root {
		t.Fatalf("expected normalized root %q, got %q", root, cfg.Root)
	}
	if cfg.DiscoveryWorkers != 1 || cfg.GitWorkers != 1 {
		t.Fatalf("expected worker counts clamped to 1, got %#v", cfg)
	}
}

func TestNormalizeRejectsEmptyRoot(t *testing.T) {
	cfg := Config{}
	if err := cfg.Normalize(); err == nil {
		t.Fatalf("expected empty root to fail")
	}
}

func TestNormalizeRejectsFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := Config{Root: root}
	if err := cfg.Normalize(); err == nil {
		t.Fatalf("expected file root to fail")
	}
}

func TestMax(t *testing.T) {
	if got := max(2, 5); got != 5 {
		t.Fatalf("unexpected max: %d", got)
	}
}
