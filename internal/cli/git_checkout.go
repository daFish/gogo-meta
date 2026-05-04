package cli

import (
	"fmt"
	"os"

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
	branch := args[0]

	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}

	configResult, err := resolveConfig()
	if err != nil {
		return err
	}

	loopOpts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	createFlag, _ := cmd.Flags().GetBool("create")

	command := fmt.Sprintf("git checkout %s", branch)
	if createFlag {
		command = fmt.Sprintf("git checkout -b %s", branch)
	}

	results, err := loop.Loop(runCtx(), command, loop.Context{
		Config:  configResult.Config,
		MetaDir: metaDir,
	}, loopOpts, newShellExecutor())
	if err != nil {
		return err
	}

	if loop.GetExitCode(results) != 0 {
		os.Exit(1)
	}
	return nil
}
