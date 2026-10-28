package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cloneExecutor records every clone invocation and answers with a per-URL
// result, tracking how many clones overlap in time.
type cloneExecutor struct {
	results map[string]*executor.Result // url -> result ("" key => default success)
	delay   time.Duration

	mu          sync.Mutex
	cloned      []string
	inFlight    int
	maxInFlight int
}

func (c *cloneExecutor) Execute(_ context.Context, _ string, _ executor.Options) (*executor.Result, error) {
	return &executor.Result{ExitCode: 0}, nil
}

func (c *cloneExecutor) ExecuteArgs(_ context.Context, _ string, args []string, _ executor.Options) (*executor.Result, error) {
	url := ""
	for i, a := range args {
		if a == "--" && i+1 < len(args) {
			url = args[i+1]
			break
		}
	}

	c.mu.Lock()
	c.cloned = append(c.cloned, url)
	c.inFlight++
	if c.inFlight > c.maxInFlight {
		c.maxInFlight = c.inFlight
	}
	c.mu.Unlock()

	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	c.mu.Lock()
	c.inFlight--
	c.mu.Unlock()

	if result, ok := c.results[url]; ok {
		return result, nil
	}
	return &executor.Result{ExitCode: 0}, nil
}

func TestCloneMissingReportsOutcomesInInputOrder(t *testing.T) {
	targets := []cloneTarget{
		{Path: "libs/web", URL: "git@x:o/web.git"},
		{Path: "api", URL: "git@x:o/api.git"},
		{Path: "libs/shared", URL: "git@x:o/shared.git"},
	}

	for _, parallel := range []bool{false, true} {
		ex := &cloneExecutor{}
		outcomes := cloneMissing(context.Background(), ex, t.TempDir(), targets, parallel, 4, nil)

		require.Len(t, outcomes, len(targets))
		for i, target := range targets {
			assert.Equal(t, target.Path, outcomes[i].Path, "parallel=%v", parallel)
			assert.Empty(t, outcomes[i].Err, "parallel=%v", parallel)
		}
	}
}

func TestCloneMissingIsolatesPerTargetFailures(t *testing.T) {
	tests := []struct {
		name    string
		targets []cloneTarget
		results map[string]*executor.Result
		wantErr map[string]string // path -> substring of the reported error ("" => cloned)
	}{
		{
			name: "invalid URL does not stop the others",
			targets: []cloneTarget{
				{Path: "api", URL: "git@x:o/api.git"},
				{Path: "bad", URL: "--upload-pack=evil"},
				{Path: "web", URL: "git@x:o/web.git"},
			},
			wantErr: map[string]string{"api": "", "bad": "must not begin with '-'", "web": ""},
		},
		{
			name: "failing clone does not stop the others",
			targets: []cloneTarget{
				{Path: "api", URL: "git@x:o/api.git"},
				{Path: "gone", URL: "git@x:o/gone.git"},
				{Path: "web", URL: "git@x:o/web.git"},
			},
			results: map[string]*executor.Result{
				"git@x:o/gone.git": {ExitCode: 128, Stderr: "repository not found"},
			},
			wantErr: map[string]string{"api": "", "gone": "repository not found", "web": ""},
		},
		{
			name: "clone failure with no stderr still reports a reason",
			targets: []cloneTarget{
				{Path: "quiet", URL: "git@x:o/quiet.git"},
			},
			results: map[string]*executor.Result{
				"git@x:o/quiet.git": {ExitCode: 1},
			},
			wantErr: map[string]string{"quiet": "clone failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &cloneExecutor{results: tt.results}
			outcomes := cloneMissing(context.Background(), ex, t.TempDir(), tt.targets, false, 0, nil)

			require.Len(t, outcomes, len(tt.targets))
			for _, o := range outcomes {
				want, ok := tt.wantErr[o.Path]
				require.True(t, ok, "unexpected outcome for %s", o.Path)
				if want == "" {
					assert.Empty(t, o.Err, "%s must be cloned", o.Path)
					continue
				}
				assert.Contains(t, o.Err, want)
			}

			// Every valid URL must still have been attempted.
			for _, target := range tt.targets {
				if strings.HasPrefix(target.URL, "-") {
					continue
				}
				assert.Contains(t, ex.cloned, target.URL,
					"a failure elsewhere must not cancel %s", target.Path)
			}
		})
	}
}

func TestCloneMissingHonorsParallelAndConcurrency(t *testing.T) {
	targets := make([]cloneTarget, 0, 6)
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		targets = append(targets, cloneTarget{Path: name, URL: "git@x:o/" + name + ".git"})
	}

	t.Run("sequential clones one at a time", func(t *testing.T) {
		ex := &cloneExecutor{delay: 5 * time.Millisecond}
		cloneMissing(context.Background(), ex, t.TempDir(), targets, false, 4, nil)
		assert.Equal(t, 1, ex.maxInFlight)
	})

	t.Run("parallel overlaps up to concurrency", func(t *testing.T) {
		ex := &cloneExecutor{delay: 20 * time.Millisecond}
		cloneMissing(context.Background(), ex, t.TempDir(), targets, true, 2, nil)
		assert.LessOrEqual(t, ex.maxInFlight, 2, "--concurrency must bound the clone pool")
		assert.Greater(t, ex.maxInFlight, 1, "--parallel must actually clone concurrently")
	})
}

// nestingExecutor clones like git does: it refuses a destination that exists
// and is not empty, and otherwise fills it. It also records every clone that
// starts while a clone into an ancestor or descendant directory is running,
// which catches the race even when git's refusal would not.
type nestingExecutor struct {
	delay time.Duration

	mu          sync.Mutex
	inFlight    map[string]bool
	overlaps    []string
	maxInFlight int
}

func (n *nestingExecutor) Execute(_ context.Context, _ string, _ executor.Options) (*executor.Result, error) {
	return &executor.Result{ExitCode: 0}, nil
}

func (n *nestingExecutor) ExecuteArgs(_ context.Context, _ string, args []string, opts executor.Options) (*executor.Result, error) {
	dest := filepath.Join(opts.Cwd, args[len(args)-1])

	n.mu.Lock()
	for running := range n.inFlight {
		if isWithin(dest, running) || isWithin(running, dest) {
			n.overlaps = append(n.overlaps, dest+" with "+running)
		}
	}
	n.inFlight[dest] = true
	n.maxInFlight = max(n.maxInFlight, len(n.inFlight))
	n.mu.Unlock()

	defer func() {
		n.mu.Lock()
		delete(n.inFlight, dest)
		n.mu.Unlock()
	}()

	if entries, err := os.ReadDir(dest); err == nil && len(entries) > 0 {
		return &executor.Result{ExitCode: 128, Stderr: "fatal: destination path already exists and is not an empty directory."}, nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dest, "HEAD"), nil, 0o644); err != nil {
		return nil, err
	}
	time.Sleep(n.delay)
	return &executor.Result{ExitCode: 0}, nil
}

func isWithin(path, dir string) bool {
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

func TestCloneMissingClonesAParentBeforeTheProjectsNestedInIt(t *testing.T) {
	// The nested projects are listed before their parents, so neither the input
	// order nor a pool that starts them all at once may decide the order.
	targets := []cloneTarget{
		{Path: "a/b/c", URL: "git@x:o/c.git"},
		{Path: "a/b", URL: "git@x:o/b.git"},
		{Path: "a", URL: "git@x:o/a.git"},
		{Path: "ab", URL: "git@x:o/ab.git"},
		{Path: "other", URL: "git@x:o/other.git"},
	}

	for _, parallel := range []bool{false, true} {
		t.Run(fmt.Sprintf("parallel=%v", parallel), func(t *testing.T) {
			ex := &nestingExecutor{delay: 20 * time.Millisecond, inFlight: map[string]bool{}}
			outcomes := cloneMissing(context.Background(), ex, t.TempDir(), targets, parallel, 4, nil)

			require.Len(t, outcomes, len(targets))
			for i, target := range targets {
				assert.Equal(t, target.Path, outcomes[i].Path, "outcomes keep the input order")
				assert.Empty(t, outcomes[i].Err, "%s must be cloned", target.Path)
			}
			assert.Empty(t, ex.overlaps, "a project must not be cloned while a project nested in it is")
			if parallel {
				assert.Greater(t, ex.maxInFlight, 1, "unrelated projects are still cloned at the same time")
			}
		})
	}
}
