package main

import (
	"fmt"
	"os"

	"github.com/puria/minos/internal/config"
	"github.com/puria/minos/internal/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd(base config.Config, version string, run func(config.Config, string) error) *cobra.Command {
	cfg := base
	cmd := &cobra.Command{
		Use:     "minos [root]",
		Short:   "Review and prune source trees containing many git repositories",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && !cmd.Flags().Changed("root") {
				cfg.Root = args[0]
			}

			if err := cfg.Normalize(); err != nil {
				return err
			}

			return run(cfg, version)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&cfg.Root, "root", cfg.Root, "Root directory to scan")
	flags.StringVar(&cfg.SummarizerCmd, "summarizer-cmd", cfg.SummarizerCmd, "External summarizer command")
	flags.BoolVar(&cfg.ShowCleanOnly, "show-clean-only", cfg.ShowCleanOnly, "Show only repos whose worktrees are clean")
	flags.BoolVar(&cfg.ShowSafeOnly, "show-safe-only", cfg.ShowSafeOnly, "Show only repos that are safe to remove")
	flags.BoolVar(&cfg.SafeRemoveRequiresNoExtraBranches, "safe-remove-requires-no-extra-branches", cfg.SafeRemoveRequiresNoExtraBranches, "Safe removal requires no local branches beyond the default branch")
	flags.BoolVar(&cfg.SafeRemoveRequiresNoLinkedWorktrees, "safe-remove-requires-no-linked-worktrees", cfg.SafeRemoveRequiresNoLinkedWorktrees, "Safe removal requires no linked worktrees")
	flags.BoolVar(&cfg.Debug, "debug", cfg.Debug, "Enable debug logging")
	return cmd
}

func execute(args []string, run func(config.Config, string) error) error {
	cmd := newRootCmd(config.Default(), version, run)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func main() {
	if err := execute(os.Args[1:], tui.Run); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
