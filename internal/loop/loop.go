package loop

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/output"
)

const DefaultConcurrency = 4

// MissingDirectoryMessage is reported for a configured project whose directory
// is absent. Without it the command is still run and the executor fails to
// change into the working directory, which surfaces as an exit code with no
// output at all.
const MissingDirectoryMessage = "directory missing — run 'gogo git update' to clone it"

// Context holds the configuration and directory for loop operations.
type Context struct {
	Config  config.MetaConfig
	MetaDir string
}

// Options configures loop behavior.
type Options struct {
	filter.Options
	Parallel    bool
	Concurrency int
}

// Result holds the outcome of a command execution in a single project.
type Result struct {
	Directory string
	Result    executor.Result
	Success   bool
	Duration  time.Duration
}

// CommandFn is a function that executes a command in a given directory.
type CommandFn func(ctx context.Context, absoluteDir, projectPath string) (*executor.Result, error)

// ShellCommand adapts a shell command string into a CommandFn that runs it via
// exec in each project directory.
func ShellCommand(exec executor.Executor, command string) CommandFn {
	return func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
		return exec.Execute(ctx, command, executor.Options{Cwd: absoluteDir})
	}
}

// ArgsCommand adapts an argv invocation into a CommandFn run via exec with no
// shell, so arguments cannot be interpreted as shell syntax.
func ArgsCommand(exec executor.Executor, name string, args ...string) CommandFn {
	return func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
		return exec.ExecuteArgs(ctx, name, args, executor.Options{Cwd: absoluteDir})
	}
}

// skipMissingDirectories reports a configured project whose directory is absent
// instead of running command in it.
func skipMissingDirectories(command CommandFn) CommandFn {
	return func(ctx context.Context, absoluteDir, projectPath string) (*executor.Result, error) {
		if _, err := os.Stat(absoluteDir); os.IsNotExist(err) {
			return &executor.Result{ExitCode: 1, Stderr: MissingDirectoryMessage}, nil
		}
		return command(ctx, absoluteDir, projectPath)
	}
}

// Loop executes command across all matching project directories.
func Loop(ctx context.Context, command CommandFn, loopCtx Context, opts Options) ([]Result, error) {
	directories := config.GetProjectPaths(loopCtx.Config)
	directories = filter.Apply(directories, opts.Options)

	if len(directories) == 0 {
		output.Warning("No projects match the specified filters")
		return nil, nil
	}

	command = skipMissingDirectories(command)

	var err error
	var results []Result
	if opts.Parallel {
		results, err = runParallel(ctx, command, directories, loopCtx, opts)
	} else {
		results, err = runSequential(ctx, command, directories, loopCtx, opts)
	}
	if err != nil {
		return nil, err
	}

	var failedProjects []string
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failedProjects = append(failedProjects, r.Directory)
		}
	}
	output.Summary(output.SummaryData{
		Success:        successCount,
		Failed:         len(results) - successCount,
		Total:          len(results),
		FailedProjects: failedProjects,
	})

	return results, nil
}

func runSequential(ctx context.Context, command CommandFn, directories []string, loopCtx Context, opts Options) ([]Result, error) {
	var results []Result

	for _, projectPath := range directories {
		absoluteDir := filepath.Join(loopCtx.MetaDir, projectPath)

		output.Header(projectPath)

		start := time.Now()

		result, err := command(ctx, absoluteDir, projectPath)
		if err != nil {
			output.CommandOutput("", err.Error())
			results = append(results, Result{
				Directory: projectPath,
				Result:    executor.Result{ExitCode: 1, Stderr: err.Error()},
				Success:   false,
				Duration:  time.Since(start),
			})
			continue
		}
		duration := time.Since(start)
		output.CommandOutput(result.Stdout, result.Stderr)
		results = append(results, Result{
			Directory: projectPath,
			Result:    *result,
			Success:   result.ExitCode == 0,
			Duration:  duration,
		})
	}

	return results, nil
}

// RunEach runs fn once per item and returns one Result per item, in input order.
// With parallel it uses a pool of at most concurrency workers (DefaultConcurrency
// when concurrency is not positive); otherwise items run in order on the calling
// goroutine. It prints nothing. A non-nil error from fn cancels the context the
// remaining items see.
func RunEach(ctx context.Context, items []string, parallel bool, concurrency int, fn func(ctx context.Context, item string) (*executor.Result, error)) []Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]Result, len(items))

	run := func(idx int) {
		start := time.Now()
		result, err := fn(ctx, items[idx])
		duration := time.Since(start)

		if err != nil {
			cancel()
			results[idx] = Result{
				Directory: items[idx],
				Result:    executor.Result{ExitCode: 1, Stderr: err.Error()},
				Success:   false,
				Duration:  duration,
			}
			return
		}

		results[idx] = Result{
			Directory: items[idx],
			Result:    *result,
			Success:   result.ExitCode == 0,
			Duration:  duration,
		}
	}

	if !parallel {
		for idx := range items {
			run(idx)
		}
		return results
	}

	workers := concurrency
	if workers <= 0 {
		workers = DefaultConcurrency
	}
	if workers > len(items) {
		workers = len(items)
	}

	work := make(chan int, len(items))
	for idx := range items {
		work <- idx
	}
	close(work)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				run(idx)
			}
		}()
	}
	wg.Wait()

	return results
}

func runParallel(ctx context.Context, command CommandFn, directories []string, loopCtx Context, opts Options) ([]Result, error) {
	results := RunEach(ctx, directories, true, opts.Concurrency, func(ctx context.Context, projectPath string) (*executor.Result, error) {
		return command(ctx, filepath.Join(loopCtx.MetaDir, projectPath), projectPath)
	})

	for _, r := range results {
		output.Header(r.Directory)
		output.CommandOutput(r.Result.Stdout, r.Result.Stderr)
	}

	return results, nil
}

// HasFailures returns true if any result has a non-zero exit code.
func HasFailures(results []Result) bool {
	for _, r := range results {
		if !r.Success {
			return true
		}
	}
	return false
}

// GetExitCode returns 1 if there are failures, 0 otherwise.
func GetExitCode(results []Result) int {
	if HasFailures(results) {
		return 1
	}
	return 0
}
