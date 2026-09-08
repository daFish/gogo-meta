package cli

import (
	"context"
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
		outcomes := cloneMissing(context.Background(), ex, t.TempDir(), targets, parallel, 4)

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
			outcomes := cloneMissing(context.Background(), ex, t.TempDir(), tt.targets, false, 0)

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
		cloneMissing(context.Background(), ex, t.TempDir(), targets, false, 4)
		assert.Equal(t, 1, ex.maxInFlight)
	})

	t.Run("parallel overlaps up to concurrency", func(t *testing.T) {
		ex := &cloneExecutor{delay: 20 * time.Millisecond}
		cloneMissing(context.Background(), ex, t.TempDir(), targets, true, 2)
		assert.LessOrEqual(t, ex.maxInFlight, 2, "--concurrency must bound the clone pool")
		assert.Greater(t, ex.maxInFlight, 1, "--parallel must actually clone concurrently")
	})
}
