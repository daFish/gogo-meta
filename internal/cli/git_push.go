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
	cmd.Flags().Bool("parallel", false, "Execute in parallel")
	return cmd
}

func runGitPush(cmd *cobra.Command, _ []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	return runLoopCommand(loop.ShellCommand(newShellExecutor(), "git push"), opts)
}
