package cli

import (
	"fmt"
	"os"

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

	deleteFlag, _ := cmd.Flags().GetBool("delete")
	allFlag, _ := cmd.Flags().GetBool("all")

	var command string
	if len(args) == 0 {
		if allFlag {
			command = "git branch -a"
		} else {
			command = "git branch"
		}
	} else {
		name := args[0]
		if deleteFlag {
			command = fmt.Sprintf("git branch -d %s", name)
		} else {
			command = fmt.Sprintf("git branch %s", name)
		}
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
