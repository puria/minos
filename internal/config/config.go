package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Root                                string
	SummarizerCmd                       string
	ShowCleanOnly                       bool
	ShowSafeOnly                        bool
	SafeRemoveRequiresNoExtraBranches   bool
	SafeRemoveRequiresNoLinkedWorktrees bool
	Debug                               bool
	DiscoveryWorkers                    int
	GitWorkers                          int
}

func Default() Config {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, "src")

	return Config{
		Root:                                root,
		SummarizerCmd:                       "codex exec --skip-git-repo-check --color never --ephemeral -",
		ShowCleanOnly:                       false,
		ShowSafeOnly:                        false,
		SafeRemoveRequiresNoExtraBranches:   true,
		SafeRemoveRequiresNoLinkedWorktrees: true,
		DiscoveryWorkers:                    max(4, runtime.NumCPU()),
		GitWorkers:                          max(4, runtime.NumCPU()),
	}
}

func (c *Config) Normalize() error {
	if c.Root == "" {
		return fmt.Errorf("root is required")
	}

	abs, err := filepath.Abs(c.Root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	c.Root = abs

	info, err := os.Stat(c.Root)
	if err != nil {
		return fmt.Errorf("stat root %q: %w", c.Root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root %q is not a directory", c.Root)
	}

	if c.DiscoveryWorkers < 1 {
		c.DiscoveryWorkers = 1
	}
	if c.GitWorkers < 1 {
		c.GitWorkers = 1
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
