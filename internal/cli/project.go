package cli

import (
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project management commands",
	}

	cmd.AddCommand(
		newProjectCreateCmd(),
		newProjectImportCmd(),
	)

	return cmd
}
