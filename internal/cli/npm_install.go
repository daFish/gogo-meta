package cli

import (
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
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	return runLoopCommand(loop.ShellCommand(newShellExecutor(), "npm install"), opts)
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
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	return runLoopCommand(loop.ShellCommand(newShellExecutor(), "npm ci"), opts)
}
