package cli

import (
	"fmt"
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

	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	opts.Parallel = false // git commit is always sequential

	escapedMessage := strings.ReplaceAll(message, `"`, `\"`)
	command := fmt.Sprintf(`git commit -m "%s"`, escapedMessage)

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
