package cli

import (
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show git status across repositories",
		RunE:  runGitStatus,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runGitStatus(cmd *cobra.Command, _ []string) error {
	return runLoopCommand(cmd, loop.ArgsCommand(newShellExecutor(), "git", "status", "--short", "--branch"))
}
