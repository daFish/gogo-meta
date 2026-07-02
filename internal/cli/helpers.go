package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func addFilterFlags(cmd *cobra.Command) {
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

func resolveFilterOptions(cmd *cobra.Command) (filter.Options, error) {
	return filter.CreateFilterOptions(
		getStringFlag(cmd, "include-only"),
		getStringFlag(cmd, "exclude-only"),
		getStringFlag(cmd, "include-pattern"),
		getStringFlag(cmd, "exclude-pattern"),
	)
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

// syncLocalExcludes keeps the gogo-meta-managed block in .git/info/exclude equal to
// the .gogo.local project set, so personal repo dirs stay out of the shared .gitignore
// and dropped projects lose their stale exclude entry.
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

// ensureLocalConfigIgnored makes sure all three .gogo.local* filenames are in the
// shared .gitignore, so a personal overlay file is never accidentally committed.
func ensureLocalConfigIgnored(metaDir string) {
	for _, name := range []string{".gogo.local", ".gogo.local.yaml", ".gogo.local.yml"} {
		_, _ = config.AddToGitignore(metaDir, name)
	}
}

func printOverlayInfo(result *config.MetaConfigResult) {
	for _, ov := range result.AppliedOverlays {
		if ov.Local {
			output.Info(fmt.Sprintf("Using local overlay config: %s", ov.Name))
		} else {
			output.Info(fmt.Sprintf("Using overlay config: %s", ov.Name))
		}
	}
}

func resolveConfig() (*config.MetaConfigResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	result, err := config.ReadMetaConfig(cwd, nil)
	if err != nil {
		return nil, err
	}
	if sibling, serr := config.UnmergedLocalSibling(cwd); serr == nil && sibling != "" {
		output.Warning(fmt.Sprintf("Local overlay %s exists but will not be merged (format differs from the primary config)", sibling))
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
