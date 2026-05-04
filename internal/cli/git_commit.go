package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
)

func newGitCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit changes across repositories",
		RunE:  runGitCommit,
	}
	cmd.Flags().StringP("message", "m", "", "Commit message (required)")
	_ = cmd.MarkFlagRequired("message")
	addFilterFlags(cmd)
	// git commit is always sequential (never parallel).
	return cmd
}

func runGitCommit(cmd *cobra.Command, _ []string) error {
	message, _ := cmd.Flags().GetString("message")

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
	// Force sequential execution for commits.
	loopOpts.Parallel = false

	// Escape quotes in commit message.
	escapedMessage := strings.ReplaceAll(message, `"`, `\"`)
	command := fmt.Sprintf(`git commit -m "%s"`, escapedMessage)

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
