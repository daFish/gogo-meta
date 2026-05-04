package loop

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/output"
)

const DefaultConcurrency = 4

// Context holds the configuration and directory for loop operations.
type Context struct {
	Config  config.MetaConfig
	MetaDir string
}

// Options configures loop behavior.
type Options struct {
	filter.Options
	Parallel       bool
	Concurrency    int
	SuppressOutput bool
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

// Loop executes a command across all matching project directories.
// The command parameter can be a string (shell command) or a CommandFn.
func Loop(ctx context.Context, command any, loopCtx Context, opts Options, exec executor.Executor) ([]Result, error) {
	directories := config.GetProjectPaths(loopCtx.Config)

	// Apply looprc filtering.
	loopRc, err := config.ReadLoopRc(loopCtx.MetaDir)
	if err == nil && loopRc != nil && len(loopRc.Ignore) > 0 {
		directories = filter.FilterFromLoopRc(directories, loopRc.Ignore)
	}

	// Apply user filters.
	directories = filter.Apply(directories, opts.Options)

	if len(directories) == 0 {
		output.Warning("No projects match the specified filters")
		return nil, nil
	}

	var results []Result
	if opts.Parallel {
		results, err = runParallel(ctx, command, directories, loopCtx, opts, exec)
	} else {
		results, err = runSequential(ctx, command, directories, loopCtx, opts, exec)
	}
	if err != nil {
		return nil, err
	}

	if !opts.SuppressOutput {
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
	}

	return results, nil
}

func runSequential(ctx context.Context, command any, directories []string, loopCtx Context, opts Options, exec executor.Executor) ([]Result, error) {
	var results []Result

	for _, projectPath := range directories {
		absoluteDir := filepath.Join(loopCtx.MetaDir, projectPath)

		if !opts.SuppressOutput {
			output.Header(projectPath)
		}

		start := time.Now()

		result, err := executeCommand(ctx, command, absoluteDir, projectPath, exec)
		if err != nil {
			return nil, err
		}

		duration := time.Since(start)

		if !opts.SuppressOutput {
			output.CommandOutput(result.Stdout, result.Stderr)
		}

		results = append(results, Result{
			Directory: projectPath,
			Result:    *result,
			Success:   result.ExitCode == 0,
			Duration:  duration,
		})
	}

	return results, nil
}

func runParallel(ctx context.Context, command any, directories []string, loopCtx Context, opts Options, exec executor.Executor) ([]Result, error) {
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	results := make([]Result, len(directories))
	work := make(chan int, len(directories))

	for i := range directories {
		work <- i
	}
	close(work)

	workers := concurrency
	if workers > len(directories) {
		workers = len(directories)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				projectPath := directories[idx]
				absoluteDir := filepath.Join(loopCtx.MetaDir, projectPath)

				start := time.Now()
				result, err := executeCommand(ctx, command, absoluteDir, projectPath, exec)
				duration := time.Since(start)

				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}

				results[idx] = Result{
					Directory: projectPath,
					Result:    *result,
					Success:   result.ExitCode == 0,
					Duration:  duration,
				}
			}
		}()
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Print output in original order.
	if !opts.SuppressOutput {
		for _, r := range results {
			output.Header(r.Directory)
			output.CommandOutput(r.Result.Stdout, r.Result.Stderr)
		}
	}

	return results, nil
}

func executeCommand(ctx context.Context, command any, absoluteDir, projectPath string, exec executor.Executor) (*executor.Result, error) {
	switch cmd := command.(type) {
	case string:
		return exec.Execute(ctx, cmd, executor.Options{Cwd: absoluteDir})
	case CommandFn:
		return cmd(ctx, absoluteDir, projectPath)
	default:
		panic("loop: command must be a string or CommandFn")
	}
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
