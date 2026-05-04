package cli

import (
	"github.com/spf13/cobra"
)

func newNpmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "npm",
		Short: "NPM operations across repositories",
	}

	cmd.AddCommand(
		newNpmInstallCmd(),
		newNpmCiCmd(),
		newNpmLinkCmd(),
		newNpmRunCmd(),
	)

	return cmd
}
