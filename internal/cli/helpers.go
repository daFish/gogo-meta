package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/loop"
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

func runCtx() context.Context {
	return context.Background()
}
