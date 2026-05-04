package cli

import (
	"os"

	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newNpmInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "install",
		Aliases: []string{"i"},
		Short:   "Run npm install across repositories",
		RunE:    runNpmInstall,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runNpmInstall(cmd *cobra.Command, _ []string) error {
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

	results, err := loop.Loop(runCtx(), "npm install", loop.Context{
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

func newNpmCiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Run npm ci across repositories",
		RunE:  runNpmCi,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runNpmCi(cmd *cobra.Command, _ []string) error {
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

	results, err := loop.Loop(runCtx(), "npm ci", loop.Context{
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
