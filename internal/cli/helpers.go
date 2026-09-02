package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/daFish/gogo-meta/internal/ssh"
	"github.com/spf13/cobra"
)

// warnUnverifiedSSHHosts warns about SSH hosts not yet in known_hosts. gogo does
// not add host keys automatically (that would defeat host-key verification), so
// the user is told to verify and add them, or let ssh prompt during clone.
func warnUnverifiedSSHHosts(urls []string) {
	hosts := ssh.UnverifiedSSHHosts(urls)
	if len(hosts) == 0 {
		return
	}
	output.Warning(fmt.Sprintf(
		"Unverified SSH host key(s): %s. gogo does not add host keys automatically; verify and add them with ssh-keyscan, or clone interactively so ssh can prompt. Clone may fail.",
		strings.Join(hosts, ", ")))
}

func addFilterFlags(cmd *cobra.Command) {
	cmd.Flags().String("group", "", "Only include projects from the named group(s) in the config (comma-separated)")
	cmd.Flags().String("include-only", "", "Only include specified directories (comma-separated)")
	cmd.Flags().String("exclude-only", "", "Exclude specified directories (comma-separated)")
	cmd.Flags().String("include-pattern", "", "Include directories matching regex pattern")
	cmd.Flags().String("exclude-pattern", "", "Exclude directories matching regex pattern")
}

func addParallelFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("parallel", false, "Execute commands in parallel")
	cmd.Flags().Int("concurrency", 0, "Max parallel processes (default: 4)")
}

func getStringFlag(cmd *cobra.Command, name string) string {
	// Try local flags first, then inherited (persistent) flags.
	val, err := cmd.Flags().GetString(name)
	if err != nil || val == "" {
		val, _ = cmd.InheritedFlags().GetString(name)
	}
	return val
}

func getBoolFlag(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		val, _ = cmd.InheritedFlags().GetBool(name)
	}
	return val
}

func getIntFlag(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	if err != nil || val == 0 {
		val, _ = cmd.InheritedFlags().GetInt(name)
	}
	return val
}

// resolveFilterOptions builds filter options from the command's filter flags.
// The merged config is only read when --group is used.
func resolveFilterOptions(cmd *cobra.Command) (filter.Options, error) {
	return resolveFilterOptionsWithConfig(cmd, nil)
}

// resolveFilterOptionsWithConfig is resolveFilterOptions for callers that
// already hold the merged config; cfg may be nil, in which case the config is
// read on demand.
func resolveFilterOptionsWithConfig(cmd *cobra.Command, cfg *config.MetaConfig) (filter.Options, error) {
	opts, err := filter.CreateFilterOptions(
		getStringFlag(cmd, "include-only"),
		getStringFlag(cmd, "exclude-only"),
		getStringFlag(cmd, "include-pattern"),
		getStringFlag(cmd, "exclude-pattern"),
	)
	if err != nil {
		return filter.Options{}, err
	}

	groupNames := filter.ParseFilterList(getStringFlag(cmd, "group"))
	if len(groupNames) == 0 {
		return opts, nil
	}

	if cfg == nil {
		result, err := resolveConfig()
		if err != nil {
			return filter.Options{}, err
		}
		cfg = &result.Config
	}

	groupPaths, err := config.ResolveGroups(*cfg, groupNames)
	if err != nil {
		return filter.Options{}, err
	}
	opts.GroupOnly = groupPaths
	return opts, nil
}

func resolveLoopOptions(cmd *cobra.Command) (loop.Options, error) {
	filterOpts, err := resolveFilterOptions(cmd)
	if err != nil {
		return loop.Options{}, err
	}
	return loop.Options{
		Options:     filterOpts,
		Parallel:    getBoolFlag(cmd, "parallel"),
		Concurrency: getIntFlag(cmd, "concurrency"),
	}, nil
}

func resolveConfig() (*config.MetaConfigResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return config.ReadMetaConfig(cwd, nil)
}

func requireMetaDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	metaDir, err := config.GetMetaDir(cwd)
	if err != nil {
		return "", err
	}
	if metaDir == "" {
		return "", fmt.Errorf("not in a gogo-meta repository. Run \"gogo init\" first")
	}
	return metaDir, nil
}

func newShellExecutor() executor.Executor {
	return executor.NewShellExecutor()
}

// runLoopCommand resolves config + meta dir and runs command across all
// projects with opts, exiting non-zero if any project failed. Shared body of
// the exec/git/npm loop commands.
func runLoopCommand(ctx context.Context, command loop.CommandFn, opts loop.Options) error {
	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}
	cfg, err := resolveConfig()
	if err != nil {
		return err
	}
	results, err := loop.Loop(ctx, command, loop.Context{
		Config:  cfg.Config,
		MetaDir: metaDir,
	}, opts)
	if err != nil {
		return err
	}
	if loop.GetExitCode(results) != 0 {
		os.Exit(1)
	}
	return nil
}
