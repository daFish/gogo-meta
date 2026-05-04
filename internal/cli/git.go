package cli

import (
	"github.com/spf13/cobra"
)

func newGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Git operations across repositories",
	}

	cmd.AddCommand(
		newGitCloneCmd(),
		newGitUpdateCmd(),
		newGitStatusCmd(),
		newGitPullCmd(),
		newGitPushCmd(),
		newGitBranchCmd(),
		newGitCheckoutCmd(),
		newGitCommitCmd(),
	)

	return cmd
}
