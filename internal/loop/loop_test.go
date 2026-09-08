package loop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExecutor records calls and returns predefined results.
type mockExecutor struct {
	results map[string]*executor.Result
	calls   []string
	mu      sync.Mutex
}

func (m *mockExecutor) Execute(_ context.Context, command string, opts executor.Options) (*executor.Result, error) {
	m.mu.Lock()
	m.calls = append(m.calls, opts.Cwd)
	m.mu.Unlock()
	if result, ok := m.results[opts.Cwd]; ok {
		return result, nil
	}
	return &executor.Result{ExitCode: 0, Stdout: "ok"}, nil
}

func (m *mockExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts executor.Options) (*executor.Result, error) {
	return m.Execute(ctx, strings.TrimSpace(name+" "+strings.Join(args, " ")), opts)
}

func newMockExecutor(results map[string]*executor.Result) *mockExecutor {
	return &mockExecutor{results: results}
}

// mkProjects creates the working-copy directory of every named project. Loop
// skips a project whose directory is absent, so a test that expects its command
// to run has to create them.
func mkProjects(t *testing.T, metaDir string, names ...string) {
	t.Helper()
	for _, name := range names {
		require.NoError(t, os.MkdirAll(filepath.Join(metaDir, name), 0o755))
	}
}

func suppressOutput() func() {
	var buf bytes.Buffer
	origWriter := output.Writer
	origErrWriter := output.ErrWriter
	output.Writer = &buf
	output.ErrWriter = &buf
	return func() {
		output.Writer = origWriter
		output.ErrWriter = origErrWriter
	}
}

func TestLoopSequential(t *testing.T) {
	restore := suppressOutput()
	defer restore()

	dir := t.TempDir()
	cfg := config.MetaConfig{
		Projects: map[string]string{"api": "url1", "web": "url2"},
		Ignore:   []string{},
	}
	mkProjects(t, dir, "api", "web")

	mock := newMockExecutor(map[string]*executor.Result{})

	results, err := Loop(context.Background(), ShellCommand(mock, "echo test"), Context{Config: cfg, MetaDir: dir}, Options{})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.True(t, results[1].Success)
	assert.Len(t, mock.calls, 2)
}

func TestLoopParallel(t *testing.T) {
	restore := suppressOutput()
	defer restore()

	dir := t.TempDir()
	cfg := config.MetaConfig{
		Projects: map[string]string{"a": "url1", "b": "url2", "c": "url3"},
		Ignore:   []string{},
	}
	mkProjects(t, dir, "a", "b", "c")

	mock := newMockExecutor(map[string]*executor.Result{})

	results, err := Loop(context.Background(), ShellCommand(mock, "echo test"), Context{Config: cfg, MetaDir: dir}, Options{Parallel: true, Concurrency: 2})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	// Results should be in original sorted order (a, b, c).
	assert.Equal(t, "a", results[0].Directory)
	assert.Equal(t, "b", results[1].Directory)
	assert.Equal(t, "c", results[2].Directory)
}

func TestLoopWithFailures(t *testing.T) {
	restore := suppressOutput()
	defer restore()

	dir := t.TempDir()
	cfg := config.MetaConfig{
		Projects: map[string]string{"api": "url1", "web": "url2"},
		Ignore:   []string{},
	}

	mkProjects(t, dir, "api", "web")

	mock := newMockExecutor(map[string]*executor.Result{})
	// Default returns success for all, override for web specifically.
	// We need the full path since the executor receives absolute paths.
	mock.results[dir+"/web"] = &executor.Result{ExitCode: 1, Stdout: "", Stderr: "error"}

	results, err := Loop(context.Background(), ShellCommand(mock, "test"), Context{Config: cfg, MetaDir: dir}, Options{})
	require.NoError(t, err)
	assert.True(t, HasFailures(results))
	assert.Equal(t, 1, GetExitCode(results))
}

func TestLoopWithFilter(t *testing.T) {
	restore := suppressOutput()
	defer restore()

	dir := t.TempDir()
	cfg := config.MetaConfig{
		Projects: map[string]string{"api": "url1", "web": "url2", "docs": "url3"},
		Ignore:   []string{},
	}
	mkProjects(t, dir, "api", "web", "docs")

	mock := newMockExecutor(map[string]*executor.Result{})

	results, err := Loop(context.Background(), ShellCommand(mock, "echo test"), Context{Config: cfg, MetaDir: dir}, Options{
		Options: filterOpts("api,web"),
	})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestLoopNoMatch(t *testing.T) {
	restore := suppressOutput()
	defer restore()

	dir := t.TempDir()
	cfg := config.MetaConfig{
		Projects: map[string]string{"api": "url1"},
		Ignore:   []string{},
	}

	mock := newMockExecutor(map[string]*executor.Result{})

	results, err := Loop(context.Background(), ShellCommand(mock, "echo test"), Context{Config: cfg, MetaDir: dir}, Options{
		Options: filterOpts("nonexistent"),
	})
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestLoopCommandFn(t *testing.T) {
	restore := suppressOutput()
	defer restore()

	dir := t.TempDir()
	cfg := config.MetaConfig{
		Projects: map[string]string{"api": "url1"},
		Ignore:   []string{},
	}
	mkProjects(t, dir, "api")

	fn := CommandFn(func(_ context.Context, absoluteDir, projectPath string) (*executor.Result, error) {
		return &executor.Result{ExitCode: 0, Stdout: "custom:" + projectPath}, nil
	})

	results, err := Loop(context.Background(), fn, Context{Config: cfg, MetaDir: dir}, Options{})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "custom:api", results[0].Result.Stdout)
}

func TestHasFailures(t *testing.T) {
	assert.False(t, HasFailures([]Result{
		{Success: true},
		{Success: true},
	}))
	assert.True(t, HasFailures([]Result{
		{Success: true},
		{Success: false},
	}))
}

func TestGetExitCode(t *testing.T) {
	assert.Equal(t, 0, GetExitCode([]Result{{Success: true}}))
	assert.Equal(t, 1, GetExitCode([]Result{{Success: false}}))
}

func TestShellCommandRunsViaExecutor(t *testing.T) {
	mock := newMockExecutor(map[string]*executor.Result{
		"/tmp": {ExitCode: 0, Stdout: "hi"},
	})
	fn := ShellCommand(mock, "echo hi")
	res, err := fn(context.Background(), "/tmp", "proj")
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "hi", res.Stdout)
}

func TestArgsCommandRunsViaExecutor(t *testing.T) {
	mock := newMockExecutor(map[string]*executor.Result{
		"/tmp": {ExitCode: 0, Stdout: "ok"},
	})
	fn := ArgsCommand(mock, "git", "status", "--short")
	res, err := fn(context.Background(), "/tmp", "proj")
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "ok", res.Stdout)
}

func filterOpts(includeOnly string) filter.Options {
	opts, _ := filter.CreateFilterOptions(includeOnly, "", "", "")
	return opts
}

func TestParallelKeepsResultsWhenOneErrors(t *testing.T) {
	var buf bytes.Buffer
	origW, origE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = origW, origE }()

	dir := t.TempDir()
	cfg := config.MetaConfig{Projects: map[string]string{
		"a": "urlA", "b": "urlB", "c": "urlC",
	}}
	mkProjects(t, dir, "a", "b", "c")

	command := CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		if projectPath == "b" {
			return nil, errors.New("boom")
		}
		return &executor.Result{ExitCode: 0}, nil
	})

	results, err := Loop(context.Background(), command,
		Context{Config: cfg, MetaDir: dir},
		Options{Parallel: true})
	require.NoError(t, err)
	require.Len(t, results, 3)

	byDir := map[string]Result{}
	for _, r := range results {
		byDir[r.Directory] = r
	}
	assert.True(t, byDir["a"].Success)
	assert.True(t, byDir["c"].Success)

	// Assert that a result with Directory=="b" exists.
	require.Contains(t, byDir, "b")
	// Assert the failure shape is recorded correctly.
	assert.False(t, byDir["b"].Success, "the erroring project is marked failed, not discarded")
	assert.Equal(t, 1, byDir["b"].Result.ExitCode)
	assert.Equal(t, "boom", byDir["b"].Result.Stderr)
}

func TestSequentialKeepsResultsWhenOneErrors(t *testing.T) {
	var buf bytes.Buffer
	origW, origE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = origW, origE }()

	dir := t.TempDir()
	cfg := config.MetaConfig{Projects: map[string]string{
		"a": "urlA", "b": "urlB", "c": "urlC",
	}}
	mkProjects(t, dir, "a", "b", "c")

	command := CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		if projectPath == "b" {
			return nil, errors.New("boom")
		}
		return &executor.Result{ExitCode: 0}, nil
	})

	results, err := Loop(context.Background(), command,
		Context{Config: cfg, MetaDir: dir},
		Options{})
	require.NoError(t, err)
	require.Len(t, results, 3)

	byDir := map[string]Result{}
	for _, r := range results {
		byDir[r.Directory] = r
	}
	assert.True(t, byDir["a"].Success)
	assert.True(t, byDir["c"].Success)

	// Assert that a result with Directory=="b" exists.
	require.Contains(t, byDir, "b")
	// Assert the failure shape is recorded correctly.
	assert.False(t, byDir["b"].Success, "the erroring project is marked failed, not discarded")
	assert.Equal(t, 1, byDir["b"].Result.ExitCode)
	assert.Equal(t, "boom", byDir["b"].Result.Stderr)
}

func TestLoopReportsMissingDirectory(t *testing.T) {
	tests := []struct {
		name     string
		parallel bool
	}{
		{name: "sequential", parallel: false},
		{name: "parallel", parallel: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := suppressOutput()
			defer restore()

			dir := t.TempDir()
			cfg := config.MetaConfig{Projects: map[string]string{"api": "url1", "web": "url2"}}
			mkProjects(t, dir, "api")

			var ran []string
			var mu sync.Mutex
			command := CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
				mu.Lock()
				ran = append(ran, projectPath)
				mu.Unlock()
				return &executor.Result{ExitCode: 0}, nil
			})

			results, err := Loop(context.Background(), command,
				Context{Config: cfg, MetaDir: dir}, Options{Parallel: tt.parallel})
			require.NoError(t, err)
			require.Len(t, results, 2)

			byDir := map[string]Result{}
			for _, r := range results {
				byDir[r.Directory] = r
			}
			assert.True(t, byDir["api"].Success)
			assert.False(t, byDir["web"].Success, "a project without a directory must fail")
			assert.Equal(t, 1, byDir["web"].Result.ExitCode)
			assert.Equal(t, MissingDirectoryMessage, byDir["web"].Result.Stderr,
				"the failure must say why, instead of being empty")
			assert.Equal(t, []string{"api"}, ran, "the command must not run for a missing directory")
		})
	}
}

func TestRunEachPreservesInputOrder(t *testing.T) {
	tests := []struct {
		name        string
		parallel    bool
		concurrency int
	}{
		{name: "sequential", parallel: false},
		{name: "parallel default concurrency", parallel: true, concurrency: 0},
		{name: "parallel concurrency 1", parallel: true, concurrency: 1},
		{name: "parallel concurrency above item count", parallel: true, concurrency: 99},
	}

	items := []string{"c", "a", "b", "d"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := RunEach(context.Background(), items, tt.parallel, tt.concurrency,
				func(_ context.Context, item string) (*executor.Result, error) {
					return &executor.Result{ExitCode: 0, Stdout: "out:" + item}, nil
				})

			require.Len(t, results, len(items))
			for i, item := range items {
				assert.Equal(t, item, results[i].Directory)
				assert.Equal(t, "out:"+item, results[i].Result.Stdout)
				assert.True(t, results[i].Success)
			}
		})
	}
}

func TestRunEachHonorsConcurrencyLimit(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f"}

	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0

	results := RunEach(context.Background(), items, true, 2,
		func(_ context.Context, _ string) (*executor.Result, error) {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			mu.Unlock()

			time.Sleep(20 * time.Millisecond)

			mu.Lock()
			inFlight--
			mu.Unlock()
			return &executor.Result{ExitCode: 0}, nil
		})

	require.Len(t, results, len(items))
	assert.LessOrEqual(t, maxInFlight, 2, "no more than concurrency items may run at once")
	assert.Greater(t, maxInFlight, 1, "parallel must actually overlap work")
}

func TestRunEachSequentialDoesNotOverlap(t *testing.T) {
	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0

	RunEach(context.Background(), []string{"a", "b", "c"}, false, 4,
		func(_ context.Context, _ string) (*executor.Result, error) {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			mu.Unlock()

			time.Sleep(5 * time.Millisecond)

			mu.Lock()
			inFlight--
			mu.Unlock()
			return &executor.Result{ExitCode: 0}, nil
		})

	assert.Equal(t, 1, maxInFlight, "sequential must run one item at a time")
}

func TestRunEachRecordsErrorAsFailedResult(t *testing.T) {
	items := []string{"a", "b"}

	results := RunEach(context.Background(), items, false, 0,
		func(_ context.Context, item string) (*executor.Result, error) {
			if item == "a" {
				return nil, errors.New("boom")
			}
			return &executor.Result{ExitCode: 0}, nil
		})

	require.Len(t, results, 2)
	assert.False(t, results[0].Success)
	assert.Equal(t, 1, results[0].Result.ExitCode)
	assert.Equal(t, "boom", results[0].Result.Stderr)
	assert.Equal(t, "a", results[0].Directory)
}

func TestRunEachEmptyItems(t *testing.T) {
	results := RunEach(context.Background(), nil, true, 4,
		func(_ context.Context, _ string) (*executor.Result, error) {
			t.Fatal("fn must not be called for an empty item list")
			return nil, nil
		})
	assert.Empty(t, results)
}
