package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executionCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addParallelFlags(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func withExecution(mode config.ExecutionMode, concurrency *int) config.MetaConfig {
	return config.MetaConfig{Execution: &config.ExecutionConfig{Mode: mode, Concurrency: concurrency}}
}

func intPtr(n int) *int { return &n }

func boolPtr(b bool) *bool { return &b }

func TestResolveExecutionModeAgainstEachConfigValue(t *testing.T) {
	configs := []struct {
		name string
		cfg  config.MetaConfig
	}{
		{"no execution block", config.MetaConfig{}},
		{"empty execution block", withExecution("", nil)},
		{"mode parallel", withExecution(config.ExecutionParallel, nil)},
		{"mode sequential", withExecution(config.ExecutionSequential, nil)},
	}
	flags := []struct {
		args []string
		// want is the expected mode per config, in the order of configs.
		want [4]bool
	}{
		{nil, [4]bool{true, true, true, false}},
		{[]string{"--parallel"}, [4]bool{true, true, true, true}},
		{[]string{"--sequential"}, [4]bool{false, false, false, false}},
		{[]string{"--parallel=false"}, [4]bool{false, false, false, false}},
		{[]string{"--sequential=false"}, [4]bool{true, true, true, true}},
	}

	for _, f := range flags {
		for i, c := range configs {
			t.Run(c.name+" with "+argsName(f.args), func(t *testing.T) {
				parallel, _, err := resolveExecution(executionCmd(t, f.args...), c.cfg, nil)
				require.NoError(t, err)
				assert.Equal(t, f.want[i], parallel)
			})
		}
	}
}

func argsName(args []string) string {
	if len(args) == 0 {
		return "no flag"
	}
	return strings.Join(args, " ")
}

func TestResolveExecution(t *testing.T) {
	sequentialDef := &config.CommandConfig{Cmd: "make deploy", Parallel: boolPtr(false)}
	parallelDef := &config.CommandConfig{Cmd: "make test", Parallel: boolPtr(true)}
	concurrencyDef := &config.CommandConfig{Cmd: "make lint", Concurrency: intPtr(3)}
	plainDef := &config.CommandConfig{Cmd: "make"}

	tests := []struct {
		name            string
		args            []string
		cfg             config.MetaConfig
		commandDef      *config.CommandConfig
		wantParallel    bool
		wantConcurrency int
	}{
		{name: "built-in defaults", wantParallel: true, wantConcurrency: loop.DefaultConcurrency},
		{name: "empty execution block", cfg: withExecution("", nil), wantParallel: true, wantConcurrency: loop.DefaultConcurrency},
		{name: "execution concurrency", cfg: withExecution("", intPtr(2)), wantParallel: true, wantConcurrency: 2},
		{name: "execution concurrency is kept in sequential mode", cfg: withExecution(config.ExecutionSequential, intPtr(2)), wantParallel: false, wantConcurrency: 2},
		{name: "--concurrency beats execution concurrency", args: []string{"--concurrency", "8"}, cfg: withExecution("", intPtr(2)), wantParallel: true, wantConcurrency: 8},
		{name: "--concurrency without a config", args: []string{"--concurrency", "1"}, wantParallel: true, wantConcurrency: 1},
		{name: "--concurrency does not change the mode", args: []string{"--concurrency", "8"}, cfg: withExecution(config.ExecutionSequential, nil), wantParallel: false, wantConcurrency: 8},
		{name: "--parallel uses execution concurrency", args: []string{"--parallel"}, cfg: withExecution(config.ExecutionSequential, intPtr(2)), wantParallel: true, wantConcurrency: 2},

		{name: "command parallel false beats execution mode parallel", cfg: withExecution(config.ExecutionParallel, nil), commandDef: sequentialDef, wantParallel: false, wantConcurrency: loop.DefaultConcurrency},
		{name: "command parallel false beats the default", commandDef: sequentialDef, wantParallel: false, wantConcurrency: loop.DefaultConcurrency},
		{name: "command parallel true beats execution mode sequential", cfg: withExecution(config.ExecutionSequential, nil), commandDef: parallelDef, wantParallel: true, wantConcurrency: loop.DefaultConcurrency},
		{name: "command without parallel falls back to execution mode", cfg: withExecution(config.ExecutionSequential, nil), commandDef: plainDef, wantParallel: false, wantConcurrency: loop.DefaultConcurrency},
		{name: "--parallel beats command parallel false", args: []string{"--parallel"}, commandDef: sequentialDef, wantParallel: true, wantConcurrency: loop.DefaultConcurrency},
		{name: "--sequential beats command parallel true", args: []string{"--sequential"}, commandDef: parallelDef, wantParallel: false, wantConcurrency: loop.DefaultConcurrency},
		{name: "command concurrency beats execution concurrency", cfg: withExecution("", intPtr(2)), commandDef: concurrencyDef, wantParallel: true, wantConcurrency: 3},
		{name: "--concurrency beats command concurrency", args: []string{"--concurrency", "5"}, commandDef: concurrencyDef, wantParallel: true, wantConcurrency: 5},
		{name: "command without concurrency falls back to execution concurrency", cfg: withExecution("", intPtr(2)), commandDef: plainDef, wantParallel: true, wantConcurrency: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parallel, concurrency, err := resolveExecution(executionCmd(t, tt.args...), tt.cfg, tt.commandDef)
			require.NoError(t, err)
			assert.Equal(t, tt.wantParallel, parallel, "mode")
			assert.Equal(t, tt.wantConcurrency, concurrency, "concurrency")
		})
	}
}

func TestResolveExecutionRejectsInvalidFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"both mode flags", []string{"--parallel", "--sequential"}, "--parallel and --sequential cannot be used together"},
		{"both mode flags, negated", []string{"--parallel=false", "--sequential=false"}, "--parallel and --sequential cannot be used together"},
		{"zero concurrency", []string{"--concurrency", "0"}, "--concurrency must be at least 1 (got 0)"},
		{"negative concurrency", []string{"--concurrency", "-2"}, "--concurrency must be at least 1 (got -2)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := resolveExecution(executionCmd(t, tt.args...), withExecution(config.ExecutionSequential, nil), nil)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

// TestResolveExecutionAgainstTheMergedConfig resolves against config files on
// disk: the execution block is merged key by key, so an overlay that sets only
// the mode keeps the concurrency of .gogo.
func TestResolveExecutionAgainstTheMergedConfig(t *testing.T) {
	const primary = `{"projects":{"api":"urlA"},"execution":{"concurrency":2}}`
	tests := []struct {
		name            string
		files           map[string]string
		overlays        []string
		args            []string
		wantParallel    bool
		wantConcurrency int
	}{
		{
			name:            ".gogo alone",
			files:           map[string]string{".gogo": primary},
			wantParallel:    true,
			wantConcurrency: 2,
		},
		{
			name:            ".gogo.local overrides the mode",
			files:           map[string]string{".gogo": primary, ".gogo.local": `{"execution":{"mode":"sequential"}}`},
			wantParallel:    false,
			wantConcurrency: 2,
		},
		{
			name:            "--parallel beats .gogo.local",
			files:           map[string]string{".gogo": primary, ".gogo.local": `{"execution":{"mode":"sequential"}}`},
			args:            []string{"--parallel"},
			wantParallel:    true,
			wantConcurrency: 2,
		},
		{
			name:            ".gogo.local overrides the concurrency",
			files:           map[string]string{".gogo": primary, ".gogo.local": `{"execution":{"concurrency":6}}`},
			wantParallel:    true,
			wantConcurrency: 6,
		},
		{
			name:            "-f overlay overrides the mode",
			files:           map[string]string{".gogo": primary, ".gogo.ci.yaml": "execution:\n  mode: sequential\n"},
			overlays:        []string{".gogo.ci.yaml"},
			wantParallel:    false,
			wantConcurrency: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
			}
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
			_ = captureOutput(t)
			initTestChdir(t, dir)
			config.SetOverlayFiles(tt.overlays)
			t.Cleanup(func() { config.SetOverlayFiles(nil) })

			_, opts, err := prepareLoop(executionCmd(t, tt.args...))
			require.NoError(t, err)
			assert.Equal(t, tt.wantParallel, opts.Parallel, "mode")
			assert.Equal(t, tt.wantConcurrency, opts.Concurrency, "concurrency")
		})
	}
}

// overlapProbe is a shell command that leaves an "overlap" file in the meta
// directory when it starts while another project is still running it.
const overlapProbe = `mkdir ../running 2>/dev/null || touch ../overlap; sleep 0.2; rmdir ../running 2>/dev/null; true`

func setupOverlapRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	for _, p := range []string{"a", "b", "c"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, p), 0o755))
	}
	config.SetOverlayFiles(nil)
	_ = captureOutput(t)
	initTestChdir(t, dir)
	return dir
}

func TestExecModeEndToEnd(t *testing.T) {
	const projects = `{"projects":{"a":"urlA","b":"urlB","c":"urlC"}}`
	tests := []struct {
		name        string
		files       map[string]string
		flags       []string
		wantOverlap bool
	}{
		{"parallel by default", map[string]string{".gogo": projects}, nil, true},
		{"--sequential", map[string]string{".gogo": projects}, []string{"--sequential"}, false},
		{"execution.mode sequential in .gogo.local", map[string]string{".gogo": projects, ".gogo.local": `{"execution":{"mode":"sequential"}}`}, nil, false},
		{"--parallel beats .gogo.local", map[string]string{".gogo": projects, ".gogo.local": `{"execution":{"mode":"sequential"}}`}, []string{"--parallel"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupOverlapRepo(t, tt.files)

			root := NewRootCommand("test")
			root.SetArgs(append(append([]string{"exec"}, tt.flags...), overlapProbe))
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			require.NoError(t, root.Execute())

			overlap := filepath.Join(dir, "overlap")
			if tt.wantOverlap {
				assert.FileExists(t, overlap, "projects run at the same time")
			} else {
				assert.NoFileExists(t, overlap, "projects run one at a time")
			}
		})
	}
}

func TestRunHonoursCommandParallel(t *testing.T) {
	tests := []struct {
		name        string
		execution   string
		parallel    string
		wantOverlap bool
	}{
		{"parallel false beats execution.mode parallel", `{"mode":"parallel"}`, "false", false},
		{"parallel true beats execution.mode sequential", `{"mode":"sequential"}`, "true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupOverlapRepo(t, map[string]string{
				".gogo": `{"projects":{"a":"urlA","b":"urlB","c":"urlC"},"execution":` + tt.execution + `,` +
					`"commands":{"probe":{"cmd":"` + overlapProbe + `","parallel":` + tt.parallel + `}}}`,
			})

			cmd := filterCmd(t, nil)
			cmd.Flags().BoolP("list", "l", false, "")
			cmd.SetContext(context.Background())
			require.NoError(t, runRun(cmd, []string{"probe"}))

			overlap := filepath.Join(dir, "overlap")
			if tt.wantOverlap {
				assert.FileExists(t, overlap, "projects run at the same time")
			} else {
				assert.NoFileExists(t, overlap, "projects run one at a time")
			}
		})
	}
}

func TestLoopCommandRejectsBothModeFlagsBeforeRunning(t *testing.T) {
	dir := setupOverlapRepo(t, map[string]string{".gogo": `{"projects":{"a":"urlA","b":"urlB","c":"urlC"}}`})
	out := captureOutput(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"exec", "--parallel", "--sequential", "touch marker"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.Execute()

	require.EqualError(t, err, "--parallel and --sequential cannot be used together")
	assert.NoFileExists(t, filepath.Join(dir, "a", "marker"))
	assert.NotContains(t, out.String(), "Executing")
}

func TestGitCommitStaysSequential(t *testing.T) {
	setupOverlapRepo(t, map[string]string{
		".gogo": `{"projects":{"a":"urlA","b":"urlB","c":"urlC"},"execution":{"mode":"parallel","concurrency":3}}`,
	})

	for _, args := range [][]string{
		{"git", "commit", "-m", "x"},
		{"git", "commit", "-m", "x", "--parallel"},
		{"--parallel", "git", "commit", "-m", "x"},
		{"git", "commit", "-m", "x", "--parallel", "--sequential"},
	} {
		t.Run(argsName(args), func(t *testing.T) {
			root := NewRootCommand("test")
			commitCmd, _, err := root.Find([]string{"git", "commit"})
			require.NoError(t, err)

			var opts loop.Options
			commitCmd.RunE = func(cmd *cobra.Command, _ []string) error {
				var err error
				_, opts, err = prepareGitCommit(cmd)
				return err
			}
			root.SetArgs(args)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			require.NoError(t, root.Execute(), "--parallel is silently ignored")
			assert.False(t, opts.Parallel)
		})
	}
}

// TestLoopCommandsDeclareExecutionFlags pins that every command running in all
// projects offers the three execution flags with the same help text, so none
// of them falls back to the root's copies with a different meaning.
func TestLoopCommandsDeclareExecutionFlags(t *testing.T) {
	root := NewRootCommand("test")
	for _, path := range [][]string{
		{"exec"},
		{"run"},
		{"update"},
		{"git", "update"},
		{"git", "status"},
		{"git", "pull"},
		{"git", "push"},
		{"git", "branch"},
		{"git", "checkout"},
		{"npm", "install"},
		{"npm", "ci"},
		{"npm", "run"},
	} {
		cmd, _, err := root.Find(path)
		require.NoError(t, err)
		for name, usage := range map[string]string{
			"parallel":    parallelFlagUsage,
			"sequential":  sequentialFlagUsage,
			"concurrency": concurrencyFlagUsage,
		} {
			flag := cmd.LocalFlags().Lookup(name)
			if assert.NotNil(t, flag, "%v declares --%s", path, name) {
				assert.Equal(t, usage, flag.Usage, "%v --%s", path, name)
			}
		}
	}
}

// TestExecutionFlagPositions pins that --parallel, --sequential and
// --concurrency reach a command from either side of the subcommand. Loop
// commands declare their own copies of the flags while the root declares them
// persistently, so the two could shadow each other; git commit only has the
// root's copies.
func TestExecutionFlagPositions(t *testing.T) {
	resolveVia := func(t *testing.T, path []string, args ...string) (bool, int) {
		t.Helper()
		root := NewRootCommand("test")
		sub, _, err := root.Find(path)
		require.NoError(t, err)

		var parallel bool
		var concurrency int
		sub.RunE = func(cmd *cobra.Command, _ []string) error {
			var err error
			parallel, concurrency, err = resolveExecution(cmd, config.MetaConfig{}, nil)
			return err
		}
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		require.NoError(t, root.Execute())
		return parallel, concurrency
	}

	gitUpdate := []string{"git", "update"}
	tests := []struct {
		name            string
		path            []string
		args            []string
		wantParallel    bool
		wantConcurrency int
	}{
		{"absent", gitUpdate, []string{"git", "update"}, true, loop.DefaultConcurrency},
		{"--sequential before the subcommand", gitUpdate, []string{"--sequential", "--concurrency", "8", "git", "update"}, false, 8},
		{"--sequential after the subcommand", gitUpdate, []string{"git", "update", "--sequential", "--concurrency", "3"}, false, 3},
		{"--parallel before the subcommand", gitUpdate, []string{"--parallel", "--concurrency", "8", "git", "update"}, true, 8},
		{"--parallel after the subcommand", gitUpdate, []string{"git", "update", "--parallel", "--concurrency", "3"}, true, 3},
		{"root flags only", []string{"git", "commit"}, []string{"git", "commit", "-m", "x", "--sequential", "--concurrency", "2"}, false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parallel, concurrency := resolveVia(t, tt.path, tt.args...)
			assert.Equal(t, tt.wantParallel, parallel)
			assert.Equal(t, tt.wantConcurrency, concurrency)
		})
	}
}

// TestUpdateCommandsResolveExecutionEndToEnd runs gogo update and gogo git
// update through their real RunE, so a command that dropped the execution
// flags or the config's execution block on the way to the clone pool fails.
func TestUpdateCommandsResolveExecutionEndToEnd(t *testing.T) {
	const projects = `"projects":{"a":"git@x:o/a.git","b":"git@x:o/b.git","c":"git@x:o/c.git",` +
		`"d":"git@x:o/d.git","e":"git@x:o/e.git","f":"git@x:o/f.git"}`
	tests := []struct {
		name            string
		execution       string
		local           string
		flags           []string
		wantErr         string
		wantMaxInFlight int
	}{
		{name: "parallel by default, bounded by execution.concurrency", execution: `{"concurrency":2}`, wantMaxInFlight: 2},
		{name: "--sequential", execution: `{"mode":"parallel"}`, flags: []string{"--sequential"}, wantMaxInFlight: 1},
		{name: "--concurrency beats execution.concurrency", execution: `{"concurrency":4}`, flags: []string{"--concurrency", "2"}, wantMaxInFlight: 2},
		{name: "execution.mode sequential in .gogo.local", execution: `{}`, local: `{"execution":{"mode":"sequential"}}`, wantMaxInFlight: 1},
		{name: "--parallel and --sequential", execution: `{}`, flags: []string{"--parallel", "--sequential"}, wantErr: "--parallel and --sequential cannot be used together"},
	}

	for _, command := range [][]string{{"update"}, {"git", "update"}} {
		for _, tt := range tests {
			t.Run(strings.Join(command, " ")+" "+tt.name, func(t *testing.T) {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
					[]byte(`{`+projects+`,"execution":`+tt.execution+`}`), 0o644))
				if tt.local != "" {
					require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"), []byte(tt.local), 0o644))
				}
				config.SetOverlayFiles(nil)
				_ = captureOutput(t)
				initTestChdir(t, dir)

				ex := &cloneExecutor{delay: 20 * time.Millisecond}
				original := newShellExecutor
				newShellExecutor = func() executor.Executor { return ex }
				t.Cleanup(func() { newShellExecutor = original })

				root := NewRootCommand("test")
				root.SetArgs(append(append([]string{}, command...), tt.flags...))
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				err := root.Execute()

				if tt.wantErr != "" {
					require.EqualError(t, err, tt.wantErr)
					assert.Empty(t, ex.cloned, "nothing is cloned when the flags are rejected")
					return
				}
				require.NoError(t, err)
				assert.Len(t, ex.cloned, 6)
				assertCloneConcurrency(t, ex.maxInFlight, tt.wantMaxInFlight)
			})
		}
	}
}
