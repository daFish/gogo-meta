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

	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	output.Info(fmt.Sprintf("Running \"npm run %s\" across repositories...", script))

	exec := newShellExecutor()
	var command loop.CommandFn
	if ifPresent, _ := cmd.Flags().GetBool("if-present"); ifPresent {
		command = func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
			if !hasNpmScript(absoluteDir, script) {
				return &executor.Result{ExitCode: 0, Stdout: fmt.Sprintf("Script %q not found, skipping", script)}, nil
			}
			return exec.ExecuteArgs(ctx, "npm", []string{"run", script}, executor.Options{Cwd: absoluteDir})
		}
	} else {
		command = loop.ArgsCommand(exec, "npm", "run", script)
	}

	return runLoopCommand(cmd.Context(), command, opts)
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
