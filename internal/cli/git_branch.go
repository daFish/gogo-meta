package cli

import (
	"fmt"

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
	cmd.Flags().Bool("parallel", false, "Execute in parallel")
	return cmd
}

func runGitBranch(cmd *cobra.Command, args []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	deleteFlag, _ := cmd.Flags().GetBool("delete")
	allFlag, _ := cmd.Flags().GetBool("all")

	var command string
	switch {
	case len(args) == 0 && allFlag:
		command = "git branch -a"
	case len(args) == 0:
		command = "git branch"
	case deleteFlag:
		command = fmt.Sprintf("git branch -d %s", args[0])
	default:
		command = fmt.Sprintf("git branch %s", args[0])
	}

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
