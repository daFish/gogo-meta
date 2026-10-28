package cli

import (
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitBranchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch [name]",
		Short: "Branch operations across repositories",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runGitBranch,
	}
	cmd.Flags().BoolP("delete", "d", false, "Delete the specified branch")
	cmd.Flags().BoolP("all", "a", false, "List all branches (local and remote)")
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runGitBranch(cmd *cobra.Command, args []string) error {
	deleteFlag, _ := cmd.Flags().GetBool("delete")
	allFlag, _ := cmd.Flags().GetBool("all")

	var gitArgs []string
	switch {
	case len(args) == 0 && allFlag:
		gitArgs = []string{"branch", "-a"}
	case len(args) == 0:
		gitArgs = []string{"branch"}
	case deleteFlag:
		gitArgs = []string{"branch", "-d", args[0]}
	default:
		gitArgs = []string{"branch", args[0]}
	}

	return runLoopCommand(cmd, loop.ArgsCommand(newShellExecutor(), "git", gitArgs...))
}
