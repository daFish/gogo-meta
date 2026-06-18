package cli

import (
	"fmt"

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

	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	output.Info(fmt.Sprintf("Executing: %s", output.Bold(command)))

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
