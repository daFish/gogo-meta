package cli

import (
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push changes across repositories",
		RunE:  runGitPush,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runGitPush(cmd *cobra.Command, _ []string) error {
	return runLoopCommand(cmd, loop.ArgsCommand(newShellExecutor(), "git", "push"))
}
