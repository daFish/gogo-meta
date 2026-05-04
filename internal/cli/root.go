package cli

import (
	"github.com/daFish/gogo-meta/internal/config"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root cobra command with all subcommands.
func NewRootCommand(version string) *cobra.Command {
	var overlayFiles []string

	rootCmd := &cobra.Command{
		Use:           "gogo",
		Short:         "A modern CLI tool for managing multi-repository projects",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			config.SetOverlayFiles(overlayFiles)
		},
	}
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.SetHelpTemplate(`{{with .Root}}{{.Version}}

{{end}}{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`)

	pf := rootCmd.PersistentFlags()
	pf.StringSliceVarP(&overlayFiles, "file", "f", nil, "Additional config file to merge (repeatable)")
	pf.String("include-only", "", "Only include specified directories (comma-separated)")
	pf.String("exclude-only", "", "Exclude specified directories (comma-separated)")
	pf.String("include-pattern", "", "Include directories matching regex pattern")
	pf.String("exclude-pattern", "", "Exclude directories matching regex pattern")
	pf.Bool("parallel", false, "Execute commands in parallel")
	pf.Int("concurrency", 0, "Max parallel processes (default: 4)")

	rootCmd.AddCommand(
		newInitCmd(),
		newExecCmd(),
		newRunCmd(),
		newValidateCmd(),
		newGitCmd(),
		newProjectCmd(),
		newNpmCmd(),
	)

	return rootCmd
}
