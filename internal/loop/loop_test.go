package loop

import (
	"bytes"
	"context"
	"errors"
	"io"
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

// syncWriter collects output written by concurrent goroutines and signals
// every write, so a test can wait for a block to appear without sleeping.
type syncWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	wrote chan struct{}
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	select {
	case w.wrote <- struct{}{}:
	default:
	}
	return n, err
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *syncWriter) waitFor(t *testing.T, what string, cond func(out string) bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !cond(w.String()) {
		select {
		case <-w.wrote:
		case <-deadline:
			t.Fatalf("timed out waiting for %s, output so far:\n%s", what, w.String())
		}
	}
}

func captureOutput(t *testing.T) *syncWriter {
	t.Helper()
	w := &syncWriter{wrote: make(chan struct{}, 1)}
	origW, origE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = w, w
	t.Cleanup(func() { output.Writer, output.ErrWriter = origW, origE })
	return w
}

// gatedProjects holds every project's command until the test releases it, so
// a test decides the order in which projects finish.
type gatedProjects struct {
	names    []string
	release  map[string]chan struct{}
	returned chan string
}

func newGatedProjects(names ...string) *gatedProjects {
	g := &gatedProjects{
		names:    names,
		release:  make(map[string]chan struct{}, len(names)),
		returned: make(chan string, len(names)),
	}
	for _, name := range names {
		g.release[name] = make(chan struct{})
	}
	return g
}

// command prints "out:<project>" and fails the projects listed in failing
// with "err:<project>" on stderr.
func (g *gatedProjects) command(failing ...string) CommandFn {
	return func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		<-g.release[projectPath]
		defer func() { g.returned <- projectPath }()
		for _, f := range failing {
			if f == projectPath {
				return &executor.Result{ExitCode: 1, Stdout: "out:" + projectPath, Stderr: "err:" + projectPath}, nil
			}
		}
		return &executor.Result{ExitCode: 0, Stdout: "out:" + projectPath}, nil
	}
}

func (g *gatedProjects) finish(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		close(g.release[name])
		select {
		case got := <-g.returned:
			require.Equal(t, name, got)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s to finish", name)
		}
	}
}

func (g *gatedProjects) releaseAll() {
	for _, name := range g.names {
		close(g.release[name])
	}
}

func (g *gatedProjects) loopContext(t *testing.T) Context {
	t.Helper()
	dir := t.TempDir()
	projects := make(map[string]string, len(g.names))
	for _, name := range g.names {
		projects[name] = "url-" + name
	}
	mkProjects(t, dir, g.names...)
	return Context{Config: config.MetaConfig{Projects: projects}, MetaDir: dir}
}

type loopOutcome struct {
	results []Result
	err     error
}

func startLoop(command CommandFn, loopCtx Context, opts Options) <-chan loopOutcome {
	outcome := make(chan loopOutcome, 1)
	go func() {
		results, err := Loop(context.Background(), command, loopCtx, opts)
		outcome <- loopOutcome{results: results, err: err}
	}()
	return outcome
}

func awaitLoop(t *testing.T, outcome <-chan loopOutcome) []Result {
	t.Helper()
	select {
	case o := <-outcome:
		require.NoError(t, o.err)
		return o.results
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the loop to return")
		return nil
	}
}

func assertBlocksInOrder(t *testing.T, out string, names ...string) {
	t.Helper()
	last := -1
	for _, name := range names {
		block := "out:" + name
		require.Equal(t, 1, strings.Count(out, block), "the block of %s must be printed exactly once", name)
		at := strings.Index(out, block)
		assert.Greater(t, at, last, "the block of %s must follow the blocks listed before it", name)
		last = at
	}
}

var orderedProjects = []string{"alpha", "bravo", "charlie", "delta"}

func TestLoopParallelPrintsInConfigOrder(t *testing.T) {
	tests := []struct {
		name       string
		finishedAs []string
	}{
		{name: "in config order", finishedAs: []string{"alpha", "bravo", "charlie", "delta"}},
		{name: "in reverse", finishedAs: []string{"delta", "charlie", "bravo", "alpha"}},
		{name: "middle first", finishedAs: []string{"charlie", "alpha", "delta", "bravo"}},
		{name: "last first", finishedAs: []string{"delta", "alpha", "bravo", "charlie"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sequentialOut := captureOutput(t)
			sequential := newGatedProjects(orderedProjects...)
			sequential.releaseAll()
			sequentialResults, err := Loop(context.Background(), sequential.command("charlie"),
				sequential.loopContext(t), Options{})
			require.NoError(t, err)

			parallelOut := captureOutput(t)
			parallel := newGatedProjects(orderedProjects...)
			outcome := startLoop(parallel.command("charlie"), parallel.loopContext(t),
				Options{Parallel: true, Concurrency: len(orderedProjects)})
			parallel.finish(t, tt.finishedAs...)
			parallelResults := awaitLoop(t, outcome)

			out := parallelOut.String()
			assertBlocksInOrder(t, out, orderedProjects...)
			assert.Equal(t, sequentialOut.String(), out,
				"parallel output, failures and summary must read exactly like a sequential run")
			assert.NotContains(t, out, "\x1b[K", "the progress line must stay off without a terminal")

			require.Len(t, parallelResults, len(orderedProjects))
			for i, name := range orderedProjects {
				assert.Equal(t, name, parallelResults[i].Directory)
				assert.Equal(t, sequentialResults[i].Success, parallelResults[i].Success)
				assert.Equal(t, sequentialResults[i].Result, parallelResults[i].Result)
			}
			assert.Equal(t, 1, GetExitCode(parallelResults))
		})
	}
}

func TestLoopParallelPrintsABlockOnceEarlierProjectsAreDone(t *testing.T) {
	out := captureOutput(t)
	g := newGatedProjects(orderedProjects...)
	outcome := startLoop(g.command(), g.loopContext(t),
		Options{Parallel: true, Concurrency: len(orderedProjects)})

	g.finish(t, "alpha")
	out.waitFor(t, "the block of alpha", func(o string) bool { return strings.Contains(o, "out:alpha") })

	g.finish(t, "bravo")
	out.waitFor(t, "the block of bravo", func(o string) bool { return strings.Contains(o, "out:bravo") })
	assert.NotContains(t, out.String(), "out:charlie")
	assert.NotContains(t, out.String(), "out:delta")

	g.finish(t, "charlie", "delta")
	awaitLoop(t, outcome)

	assertBlocksInOrder(t, out.String(), orderedProjects...)
	assert.Contains(t, out.String(), "All 4 projects completed successfully")
}

func TestLoopParallelSlowFirstProjectHoldsBackTheRest(t *testing.T) {
	out := captureOutput(t)
	g := newGatedProjects(orderedProjects...)
	outcome := startLoop(g.command(), g.loopContext(t),
		Options{Parallel: true, Concurrency: len(orderedProjects)})

	g.finish(t, "bravo", "charlie", "delta")
	assert.NotContains(t, out.String(), "out:", "no block may be printed while alpha is still running")

	g.finish(t, "alpha")
	results := awaitLoop(t, outcome)

	assertBlocksInOrder(t, out.String(), orderedProjects...)
	require.Len(t, results, len(orderedProjects))
	assert.Equal(t, 0, GetExitCode(results))
}

func TestLoopParallelShowsTheProgressLineOnATerminal(t *testing.T) {
	out := captureOutput(t)
	isInteractive := output.IsInteractive
	output.IsInteractive = func(io.Writer) bool { return true }
	t.Cleanup(func() { output.IsInteractive = isInteractive })

	g := newGatedProjects(orderedProjects...)
	outcome := startLoop(g.command(), g.loopContext(t),
		Options{Parallel: true, Concurrency: len(orderedProjects)})

	g.finish(t, "bravo", "charlie", "delta")
	out.waitFor(t, "the progress line", func(o string) bool {
		return strings.Contains(o, "Running 3/4, waiting on alpha")
	})
	assert.NotContains(t, out.String(), "out:", "no block may be printed while alpha is still running")

	g.finish(t, "alpha")
	awaitLoop(t, outcome)

	printed := out.String()
	assertBlocksInOrder(t, printed, orderedProjects...)
	summary := strings.Index(printed, "All 4 projects completed successfully")
	require.GreaterOrEqual(t, summary, 0)
	lastClear := strings.LastIndex(printed, "\r\x1b[K")
	assert.Less(t, lastClear, summary, "the line must be cleared before the summary")
	assert.NotContains(t, printed[lastClear:], "waiting on", "the line must not be drawn again once it is cleared")
}
