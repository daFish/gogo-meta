package cli

import (
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout <branch>",
		Short: "Checkout a branch across repositories",
		Args:  cobra.ExactArgs(1),
		RunE:  runGitCheckout,
	}
	cmd.Flags().BoolP("create", "b", false, "Create the branch if it does not exist")
	addFilterFlags(cmd)
	cmd.Flags().Bool("parallel", false, "Execute in parallel")
	return cmd
}

func runGitCheckout(cmd *cobra.Command, args []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	gitArgs := []string{"checkout", args[0]}
	if create, _ := cmd.Flags().GetBool("create"); create {
		gitArgs = []string{"checkout", "-b", args[0]}
	}

	return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", gitArgs...), opts)
}
