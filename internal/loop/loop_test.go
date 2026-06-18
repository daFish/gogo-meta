package loop

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

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

func newMockExecutor(results map[string]*executor.Result) *mockExecutor {
	return &mockExecutor{results: results}
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

func filterOpts(includeOnly string) filter.Options {
	opts, _ := filter.CreateFilterOptions(includeOnly, "", "", "")
	return opts
}

func TestParallelKeepsResultsWhenOneErrors(t *testing.T) {
	var buf bytes.Buffer
	origW, origE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = origW, origE }()

	cfg := config.MetaConfig{Projects: map[string]string{
		"a": "urlA", "b": "urlB", "c": "urlC",
	}}

	command := CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		if projectPath == "b" {
			return nil, errors.New("boom")
		}
		return &executor.Result{ExitCode: 0}, nil
	})

	results, err := Loop(context.Background(), command,
		Context{Config: cfg, MetaDir: t.TempDir()},
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

	cfg := config.MetaConfig{Projects: map[string]string{
		"a": "urlA", "b": "urlB", "c": "urlC",
	}}

	command := CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		if projectPath == "b" {
			return nil, errors.New("boom")
		}
		return &executor.Result{ExitCode: 0}, nil
	})

	results, err := Loop(context.Background(), command,
		Context{Config: cfg, MetaDir: t.TempDir()},
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
