package cli

import (
	"os"

	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show git status across repositories",
		RunE:  runGitStatus,
	}
	addFilterFlags(cmd)
	cmd.Flags().Bool("parallel", false, "Execute in parallel")
	return cmd
}

func runGitStatus(cmd *cobra.Command, _ []string) error {
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

	results, err := loop.Loop(runCtx(), "git status --short --branch", loop.Context{
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
