package cli

import (
	"fmt"
	"os"

	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Execute a command in all project directories",
		Args:  cobra.ExactArgs(1),
		RunE:  runExec,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runExec(cmd *cobra.Command, args []string) error {
	command := args[0]

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

	output.Info(fmt.Sprintf("Executing: %s", output.Bold(command)))

	results, err := loop.Loop(runCtx(), command, loop.Context{
		Config:  configResult.Config,
		MetaDir: metaDir,
	}, loopOpts, newShellExecutor())
	if err != nil {
		return err
	}

	exitCode := loop.GetExitCode(results)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
