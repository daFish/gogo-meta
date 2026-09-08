package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/giturl"
	"github.com/daFish/gogo-meta/internal/loop"
)

// cloneTarget is a configured project that is absent from the working copy.
type cloneTarget struct {
	Path string
	URL  string
}

// cloneOutcome reports what happened to one cloneTarget. An empty Err means the
// repository was cloned.
type cloneOutcome struct {
	Path string
	Err  string
}

// cloneMissing clones every target into metaDir and returns one outcome per
// target, in input order. Cloning runs through loop.RunEach, so it honors
// parallel and concurrency the same way the loop commands do.
func cloneMissing(ctx context.Context, ex executor.Executor, metaDir string, targets []cloneTarget, parallel bool, concurrency int) []cloneOutcome {
	paths := make([]string, len(targets))
	byPath := make(map[string]cloneTarget, len(targets))
	for i, t := range targets {
		paths[i] = t.Path
		byPath[t.Path] = t
	}

	// Per-target failures are returned as a failed Result rather than an error:
	// an error would cancel the context the remaining targets share, and one bad
	// URL must not stop the other repositories from being cloned.
	failed := func(err error) (*executor.Result, error) {
		return &executor.Result{ExitCode: 1, Stderr: err.Error()}, nil
	}

	results := loop.RunEach(ctx, paths, parallel, concurrency, func(ctx context.Context, projectPath string) (*executor.Result, error) {
		target := byPath[projectPath]

		if err := giturl.Validate(target.URL); err != nil {
			return failed(err)
		}

		projectDir := filepath.Join(metaDir, target.Path)
		parentDir := filepath.Dir(projectDir)
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return failed(err)
		}

		result, err := ex.ExecuteArgs(ctx, "git", []string{"clone", "--", target.URL, filepath.Base(target.Path)}, executor.Options{Cwd: parentDir})
		if err != nil {
			return failed(err)
		}
		return result, nil
	})

	outcomes := make([]cloneOutcome, len(targets))
	for i, r := range results {
		outcomes[i] = cloneOutcome{Path: targets[i].Path}
		if r.Success {
			continue
		}
		msg := r.Result.Stderr
		if msg == "" {
			msg = "clone failed"
		}
		outcomes[i].Err = msg
	}
	return outcomes
}
