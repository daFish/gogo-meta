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
	cmd.Flags().Bool("parallel", false, "Execute in parallel")
	return cmd
}

func runGitStatus(cmd *cobra.Command, _ []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", "status", "--short", "--branch"), opts)
}
