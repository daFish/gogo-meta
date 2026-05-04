package cli

import (
	"os"

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

	results, err := loop.Loop(runCtx(), "git push", loop.Context{
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
