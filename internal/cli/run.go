package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Run a predefined command from .gogo file",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRun,
	}
	cmd.Flags().BoolP("list", "l", false, "List all available commands")
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	_, err := requireMetaDir()
	if err != nil {
		return err
	}

	configResult, err := resolveConfig()
	if err != nil {
		return err
	}

	listFlag, _ := cmd.Flags().GetBool("list")

	if listFlag || len(args) == 0 {
		formatCommandList(config.ListCommands(configResult.Config))
		return nil
	}

	name := args[0]
	commandDef, ok := config.GetCommand(configResult.Config, name)
	if !ok {
		available := make([]string, 0)
		if configResult.Config.Commands != nil {
			for k := range configResult.Config.Commands {
				available = append(available, k)
			}
		}
		if len(available) == 0 {
			return fmt.Errorf("unknown command: %q. No commands are defined in .gogo file", name)
		}
		return fmt.Errorf("unknown command: %q. Available commands: %s", name, strings.Join(available, ", "))
	}

	cliFilterOpts, err := resolveFilterOptions(cmd)
	if err != nil {
		return err
	}

	// Merge CLI filters with command definition filters.
	mergedFilter := cliFilterOpts
	if len(mergedFilter.IncludeOnly) == 0 && len(commandDef.IncludeOnly) > 0 {
		mergedFilter.IncludeOnly = commandDef.IncludeOnly
	}
	if len(mergedFilter.ExcludeOnly) == 0 && len(commandDef.ExcludeOnly) > 0 {
		mergedFilter.ExcludeOnly = commandDef.ExcludeOnly
	}
	if mergedFilter.IncludePattern == nil && commandDef.IncludePattern != "" {
		mergedFilter.IncludePattern, _ = filter.ParseFilterPattern(commandDef.IncludePattern)
	}
	if mergedFilter.ExcludePattern == nil && commandDef.ExcludePattern != "" {
		mergedFilter.ExcludePattern, _ = filter.ParseFilterPattern(commandDef.ExcludePattern)
	}

	parallel := getBoolFlag(cmd, "parallel")
	if !parallel && commandDef.Parallel != nil && *commandDef.Parallel {
		parallel = true
	}
	concurrency := getIntFlag(cmd, "concurrency")
	if concurrency == 0 && commandDef.Concurrency != nil {
		concurrency = *commandDef.Concurrency
	}

	loopOpts := loop.Options{
		Options:     mergedFilter,
		Parallel:    parallel,
		Concurrency: concurrency,
	}

	metaDir, _ := requireMetaDir()
	output.Info(fmt.Sprintf("Running %q: %s", name, output.Bold(commandDef.Cmd)))

	results, err := loop.Loop(runCtx(), commandDef.Cmd, loop.Context{
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

func formatCommandList(commands []config.CommandEntry) {
	if len(commands) == 0 {
		output.Info("No commands defined in .gogo file")
		output.Dim("  Add commands to your .gogo file:")
		output.Dim("  \"commands\": { \"build\": \"npm run build\" }")
		return
	}

	output.Info("Available commands:")
	_, _ = fmt.Fprintln(output.Writer)

	maxLen := 0
	for i := range commands {
		if len(commands[i].Name) > maxLen {
			maxLen = len(commands[i].Name)
		}
	}

	for i := range commands {
		desc := commands[i].Command.Description
		if desc == "" {
			desc = commands[i].Command.Cmd
		}
		padded := commands[i].Name + strings.Repeat(" ", maxLen-len(commands[i].Name))
		_, _ = fmt.Fprintf(output.Writer, "  %s  %s\n", output.Bold(padded), desc)
	}
}
