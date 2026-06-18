package cli

import (
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull changes across repositories",
		RunE:  runGitPull,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runGitPull(cmd *cobra.Command, _ []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	return runLoopCommand(loop.ShellCommand(newShellExecutor(), "git pull"), opts)
}
