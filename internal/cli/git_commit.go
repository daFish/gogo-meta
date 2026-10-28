package cli

import (
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
	return cmd
}

func runGitCommit(cmd *cobra.Command, _ []string) error {
	message, _ := cmd.Flags().GetString("message")

	loopCtx, opts, err := prepareGitCommit(cmd)
	if err != nil {
		return err
	}
	return runLoop(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", "commit", "-m", message), loopCtx, opts)
}

// prepareGitCommit is prepareLoop without the execution flags and config:
// commit hooks and signing assume one commit at a time, so git commit always
// runs sequentially and ignores --parallel.
func prepareGitCommit(cmd *cobra.Command) (loop.Context, loop.Options, error) {
	loopCtx, err := loadLoopContext()
	if err != nil {
		return loop.Context{}, loop.Options{}, err
	}
	filterOpts, err := resolveFilterOptionsWithConfig(cmd, &loopCtx.Config)
	if err != nil {
		return loop.Context{}, loop.Options{}, err
	}
	return loopCtx, loop.Options{Options: filterOpts, Parallel: false}, nil
}
