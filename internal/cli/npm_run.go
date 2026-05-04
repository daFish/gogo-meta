package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newNpmRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <script>",
		Short: "Run an npm script across repositories",
		Args:  cobra.ExactArgs(1),
		RunE:  runNpmRun,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	cmd.Flags().Bool("if-present", false, "Only run if script exists in project's package.json")
	return cmd
}

func runNpmRun(cmd *cobra.Command, args []string) error {
	script := args[0]

	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}

	configResult, err := resolveConfig()
	if err != nil {
		return err
	}

	loopOpts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	ifPresent, _ := cmd.Flags().GetBool("if-present")

	output.Info(fmt.Sprintf("Running \"npm run %s\" across repositories...", script))

	var command any
	if ifPresent {
		exec := executor.NewShellExecutor()
		command = loop.CommandFn(func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
			if !hasNpmScript(absoluteDir, script) {
				return &executor.Result{
					ExitCode: 0,
					Stdout:   fmt.Sprintf("Script %q not found, skipping", script),
				}, nil
			}
			return exec.Execute(ctx, fmt.Sprintf("npm run %s", script), executor.Options{Cwd: absoluteDir})
		})
	} else {
		command = fmt.Sprintf("npm run %s", script)
	}

	results, err := loop.Loop(runCtx(), command, loop.Context{
		Config:  configResult.Config,
		MetaDir: metaDir,
	}, loopOpts, newShellExecutor())
	if err != nil {
		return err
	}

	if loop.GetExitCode(results) != 0 {
		os.Exit(1)
	}
	return nil
}

func hasNpmScript(dir, scriptName string) bool {
	pkgPath := filepath.Join(dir, "package.json")
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return false
	}

	_, ok := pkg.Scripts[scriptName]
	return ok
}
