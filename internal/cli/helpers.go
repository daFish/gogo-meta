package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
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

const (
	parallelFlagUsage    = "Run in parallel (default unless the config sets execution.mode: sequential)"
	sequentialFlagUsage  = "Run one project at a time"
	concurrencyFlagUsage = "Max parallel processes (default: 4, or execution.concurrency)"
)

func addParallelFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("parallel", false, parallelFlagUsage)
	cmd.Flags().Bool("sequential", false, sequentialFlagUsage)
	cmd.Flags().Int("concurrency", 0, concurrencyFlagUsage)
}

// executionFlags holds what --parallel, --sequential and --concurrency asked
// for. A nil field was not passed. --parallel=false asks for sequential mode
// and --sequential=false for parallel mode.
type executionFlags struct {
	parallel    *bool
	concurrency *int
}

// passedFlag returns the value of the flag named name if it was passed, whether
// it is the command's own flag or one inherited from the root.
func passedFlag(cmd *cobra.Command, name string) (value string, passed bool) {
	if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
		return flag.Value.String(), true
	}
	if flag := cmd.InheritedFlags().Lookup(name); flag != nil && flag.Changed {
		return flag.Value.String(), true
	}
	return "", false
}

func readExecutionFlags(cmd *cobra.Command) (executionFlags, error) {
	var flags executionFlags

	parallelValue, parallelPassed := passedFlag(cmd, "parallel")
	sequentialValue, sequentialPassed := passedFlag(cmd, "sequential")
	switch {
	case parallelPassed && sequentialPassed:
		return executionFlags{}, errors.New("--parallel and --sequential cannot be used together")
	case parallelPassed:
		parallel, err := strconv.ParseBool(parallelValue)
		if err != nil {
			return executionFlags{}, err
		}
		flags.parallel = &parallel
	case sequentialPassed:
		sequential, err := strconv.ParseBool(sequentialValue)
		if err != nil {
			return executionFlags{}, err
		}
		parallel := !sequential
		flags.parallel = &parallel
	}

	if value, passed := passedFlag(cmd, "concurrency"); passed {
		concurrency, err := strconv.Atoi(value)
		if err != nil {
			return executionFlags{}, err
		}
		if concurrency < 1 {
			return executionFlags{}, fmt.Errorf("--concurrency must be at least 1 (got %d)", concurrency)
		}
		flags.concurrency = &concurrency
	}

	return flags, nil
}

// resolve decides the mode and the concurrency, each from the first source
// that sets it: the flags, the command definition of gogo run (commandDef may
// be nil), the execution block of the merged config, and the built-in default
// of parallel mode with loop.DefaultConcurrency workers.
func (f executionFlags) resolve(cfg config.MetaConfig, commandDef *config.CommandConfig) (parallel bool, concurrency int) {
	switch {
	case f.parallel != nil:
		parallel = *f.parallel
	case commandDef != nil && commandDef.Parallel != nil:
		parallel = *commandDef.Parallel
	default:
		configParallel, set := cfg.Execution.Parallel()
		parallel = configParallel || !set
	}

	switch {
	case f.concurrency != nil:
		concurrency = *f.concurrency
	case commandDef != nil && commandDef.Concurrency != nil:
		concurrency = *commandDef.Concurrency
	default:
		configConcurrency, set := cfg.Execution.MaxConcurrency()
		concurrency = loop.DefaultConcurrency
		if set {
			concurrency = configConcurrency
		}
	}

	return parallel, concurrency
}

// resolveExecution reads the execution flags of cmd and resolves them against
// the merged config and, for gogo run, the command definition.
func resolveExecution(cmd *cobra.Command, cfg config.MetaConfig, commandDef *config.CommandConfig) (parallel bool, concurrency int, err error) {
	flags, err := readExecutionFlags(cmd)
	if err != nil {
		return false, 0, err
	}
	parallel, concurrency = flags.resolve(cfg, commandDef)
	return parallel, concurrency, nil
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
		// Plumbing read, not the command's own config load: the caller resolves
		// the config again and announces the overlays there, so stay quiet to
		// avoid printing the overlay lines twice.
		result, err := readConfig()
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

// resolveLoopOptions resolves the filter and execution flags of cmd against
// the merged config.
func resolveLoopOptions(cmd *cobra.Command, cfg config.MetaConfig) (loop.Options, error) {
	filterOpts, err := resolveFilterOptionsWithConfig(cmd, &cfg)
	if err != nil {
		return loop.Options{}, err
	}
	parallel, concurrency, err := resolveExecution(cmd, cfg, nil)
	if err != nil {
		return loop.Options{}, err
	}
	return loop.Options{
		Options:     filterOpts,
		Parallel:    parallel,
		Concurrency: concurrency,
	}, nil
}

func syncLocalExcludes(metaDir string, localProjects map[string]string) error {
	paths := make([]string, 0, len(localProjects))
	for p := range localProjects {
		paths = append(paths, p)
	}
	changed, err := config.SyncGitExcludeManagedBlock(metaDir, paths)
	if err != nil {
		return err
	}
	if changed {
		output.Info("Updated .git/info/exclude for .gogo.local project directories")
	}
	return nil
}

func ensureLocalConfigIgnored(metaDir string) error {
	for _, name := range config.LocalOverlayNames {
		if _, err := config.AddToGitignore(metaDir, name); err != nil {
			return fmt.Errorf("adding %s to .gitignore: %w", name, err)
		}
	}
	return nil
}

func printOverlayInfo(result *config.MetaConfigResult) {
	for _, w := range result.Warnings {
		output.Warning(w)
	}
	for _, ov := range result.AppliedOverlays {
		if ov.Local {
			output.Info(fmt.Sprintf("Using local overlay config: %s", ov.Name))
		} else {
			output.Info(fmt.Sprintf("Using overlay config: %s", ov.Name))
		}
	}
}

// readConfig reads the merged config without printing anything about it.
func readConfig() (*config.MetaConfigResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return config.ReadMetaConfig(cwd, nil)
}

// resolveConfig is readConfig for a command's own config load: it also reports
// which overlays were merged, and any warning the read produced.
func resolveConfig() (*config.MetaConfigResult, error) {
	result, err := readConfig()
	if err != nil {
		return nil, err
	}
	printOverlayInfo(result)
	return result, nil
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

// newShellExecutor creates the executor the commands run in projects with.
// Override in tests.
var newShellExecutor = func() executor.Executor {
	return executor.NewShellExecutor()
}

// runLoopCommand runs command across all projects with the options of cmd,
// exiting non-zero if any project failed. Shared body of the exec/git/npm loop
// commands.
func runLoopCommand(cmd *cobra.Command, command loop.CommandFn) error {
	loopCtx, opts, err := prepareLoop(cmd)
	if err != nil {
		return err
	}
	return runLoop(cmd.Context(), command, loopCtx, opts)
}

// prepareLoop loads the config before anything is printed, so a rejected
// config stops a command before it announces or runs anything, and then
// resolves the options of cmd against it.
func prepareLoop(cmd *cobra.Command) (loop.Context, loop.Options, error) {
	loopCtx, err := loadLoopContext()
	if err != nil {
		return loop.Context{}, loop.Options{}, err
	}
	opts, err := resolveLoopOptions(cmd, loopCtx.Config)
	if err != nil {
		return loop.Context{}, loop.Options{}, err
	}
	return loopCtx, opts, nil
}

func loadLoopContext() (loop.Context, error) {
	metaDir, err := requireMetaDir()
	if err != nil {
		return loop.Context{}, err
	}
	cfg, err := resolveConfig()
	if err != nil {
		return loop.Context{}, err
	}
	return loop.Context{Config: cfg.Config, MetaDir: metaDir}, nil
}

func runLoop(ctx context.Context, command loop.CommandFn, loopCtx loop.Context, opts loop.Options) error {
	results, err := loop.Loop(ctx, command, loopCtx, opts)
	if err != nil {
		return err
	}
	if loop.GetExitCode(results) != 0 {
		os.Exit(1)
	}
	return nil
}
