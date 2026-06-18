# Go Rewrite Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove port residue (dead streaming code, dead `SuppressOutput` flag), make `loop.Loop` type-safe (kill `command any` + panic), de-duplicate the command layer, route `ssh` through the `Executor` interface, and fix the parallel result-discarding bug — with no observable behavior change except the bug fix.

**Architecture:** Five sequenced tasks. Dead-code deletions first (independent), then the typed-`Loop` + command-layer de-dup (the core refactor), then `ssh` dependency injection, then the parallel/sequential failure-handling fix. The JS `main` implementation remains the behavioral reference; deletions remove code JS also doesn't use.

**Tech Stack:** Go 1.24+, cobra, testify, golangci-lint v2.

## Global Constraints

- No observable behavior change except Task 5 (parallel/sequential failure handling).
- The JS implementation on `main` is the behavioral reference; the deleted symbols are unused there too.
- Idiomatic Go; `internal/` only; errcheck/staticcheck/gocritic/gosec; `make lint` (0 issues) and `make test` green before each commit.
- Run focused tests while iterating; full `make test` once before committing.
- **Commit style for this repo: headline only — NO `Issue:` trailer, NO `Co-Authored-By:` trailer.** `git commit -m "type(scope): subject"`. Never push.
- Task order matters: Task 2 (delete `SuppressOutput`) precedes Task 3 (which rewrites `loop.go`); Task 3 precedes Task 5 (which edits the runners Task 3 reshapes).

---

## Task 1: Delete the dead streaming subsystem

**Files:**
- Modify: `internal/executor/executor.go` (delete `ExecuteStreaming`, `StreamingOptions`)
- Modify: `internal/executor/executor_test.go` (delete `TestExecuteStreaming`)

**Interfaces:**
- Consumes: nothing.
- Produces: smaller `executor` package; `Executor` interface and `Execute`/`ExecuteSync` unchanged.

- [ ] **Step 1: Confirm no production caller**

Run: `grep -rn 'ExecuteStreaming\|StreamingOptions' internal/ cmd/ --include="*.go" | grep -v _test.go`
Expected: no output (only the definitions, which the grep’s `func`/`type` lines — re-check by eye that nothing outside `executor.go` references them).

- [ ] **Step 2: Delete the code**

In `internal/executor/executor.go`, delete the entire `StreamingOptions` type and the entire `ExecuteStreaming` function. In `internal/executor/executor_test.go`, delete `TestExecuteStreaming` (and any helper only it used). Remove now-unused imports (e.g. if `context` is still used by `Execute`, keep it).

- [ ] **Step 3: Build + test**

Run: `make lint && make test`
Expected: 0 lint issues; all packages PASS. `grep -rn 'ExecuteStreaming\|StreamingOptions' internal/` returns nothing.

- [ ] **Step 4: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "refactor(executor): remove unused streaming subsystem"
```

---

## Task 2: Delete the dead `SuppressOutput` flag

**Files:**
- Modify: `internal/loop/loop.go` (remove field + four guards)

**Interfaces:**
- Consumes: nothing.
- Produces: `loop.Options` without `SuppressOutput`; output (`Header`, `CommandOutput`, `Summary`) always runs.

- [ ] **Step 1: Confirm it is never set true**

Run: `grep -rn 'SuppressOutput' internal/ --include="*.go"`
Expected: only the field definition and the four `if !opts.SuppressOutput` reads in `loop.go` — no assignment to `true` anywhere.

- [ ] **Step 2: Remove the field and unwrap the guards**

In `internal/loop/loop.go`:
- Delete `SuppressOutput bool` from the `Options` struct.
- In `Loop`, replace the guarded summary block so it always runs:

```go
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
```

- In `runSequential`, remove the two `if !opts.SuppressOutput {` wrappers so `output.Header(projectPath)` and `output.CommandOutput(result.Stdout, result.Stderr)` always run.
- In `runParallel`, remove the `if !opts.SuppressOutput {` wrapper around the final print loop so it always runs.

- [ ] **Step 3: Build + test**

Run: `make lint && make test`
Expected: 0 issues; all PASS. `grep -rn 'SuppressOutput' internal/` returns nothing.

- [ ] **Step 4: Commit**

```bash
git add internal/loop/loop.go
git commit -m "refactor(loop): remove dead SuppressOutput flag"
```

---

## Task 3: Type-safe `Loop` + de-duplicated command layer

This is the core refactor: replace `command any` + `panic` with a typed `CommandFn`, add a `ShellCommand` adapter, drop the now-unused `exec` parameter threaded through the runners, add a `runLoopCommand` helper, and collapse the 10 near-identical command bodies. Done as one task because the `Loop` signature change forces every caller to change anyway — rewriting each caller once (into final form) avoids editing 10 files twice.

**Files:**
- Modify: `internal/loop/loop.go` (signature, `ShellCommand`, delete `executeCommand`)
- Modify: `internal/loop/loop_test.go` (update call sites; add `ShellCommand` test)
- Modify: `internal/cli/helpers.go` (add `runLoopCommand`)
- Modify: `internal/cli/{exec,git_status,git_pull,git_push,git_branch,git_checkout,git_commit,npm_install,npm_run,run}.go`
- Modify: `internal/cli/helpers_test.go` (add `runLoopCommand` test)

**Interfaces:**
- Consumes: `executor.Executor` (from `ShellCommand` call sites), existing `CommandFn`.
- Produces:
  - `func Loop(ctx context.Context, command CommandFn, loopCtx Context, opts Options) ([]Result, error)` (no `exec` param)
  - `func ShellCommand(exec executor.Executor, command string) CommandFn`
  - `func runLoopCommand(command loop.CommandFn, opts loop.Options) error` (in package `cli`)

- [ ] **Step 1: Add the `ShellCommand` test (RED)**

Add to `internal/loop/loop_test.go`:

```go
func TestShellCommandRunsViaExecutor(t *testing.T) {
	mock := newMockExecutor(map[string]*executor.Result{
		"echo hi": {ExitCode: 0, Stdout: "hi"},
	})
	fn := ShellCommand(mock, "echo hi")
	res, err := fn(context.Background(), "/tmp", "proj")
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "hi", res.Stdout)
}
```

(`newMockExecutor` already exists in `loop_test.go`; confirm its keying — it maps command string → result.)

- [ ] **Step 2: Run it (expect FAIL)**

Run: `go test ./internal/loop/ -run TestShellCommandRunsViaExecutor -v`
Expected: FAIL — `ShellCommand` undefined.

- [ ] **Step 3: Rewrite `loop.go` to the typed API**

In `internal/loop/loop.go`:

Change `Loop` to take a `CommandFn` and drop `exec`:

```go
// Loop executes command across all matching project directories.
func Loop(ctx context.Context, command CommandFn, loopCtx Context, opts Options) ([]Result, error) {
	directories := config.GetProjectPaths(loopCtx.Config)
	directories = filter.Apply(directories, opts.Options)

	if len(directories) == 0 {
		output.Warning("No projects match the specified filters")
		return nil, nil
	}

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
```

Change `runSequential` and `runParallel` to drop the `exec` parameter and call `command(...)` directly instead of `executeCommand(...)`:

- `runSequential(ctx context.Context, command CommandFn, directories []string, loopCtx Context, opts Options) ([]Result, error)` — replace `result, err := executeCommand(ctx, command, absoluteDir, projectPath, exec)` with `result, err := command(ctx, absoluteDir, projectPath)`.
- `runParallel(ctx context.Context, command CommandFn, directories []string, loopCtx Context, opts Options) ([]Result, error)` — replace `result, err := executeCommand(ctx, command, absoluteDir, projectPath, exec)` with `result, err := command(ctx, absoluteDir, projectPath)`.

Delete the entire `executeCommand` function (and its `panic`).

Add `ShellCommand` (keep `CommandFn` where it is; add `executor` import if not present — it is, via `Result`):

```go
// ShellCommand adapts a shell command string into a CommandFn that runs it via
// exec in each project directory.
func ShellCommand(exec executor.Executor, command string) CommandFn {
	return func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
		return exec.Execute(ctx, command, executor.Options{Cwd: absoluteDir})
	}
}
```

Update the doc comment on `CommandFn`/`Loop` to drop the "can be a string or a CommandFn" line.

- [ ] **Step 4: Update `loop_test.go` call sites**

Every `Loop(...)` call in `loop_test.go` currently passes a string command and a trailing `exec`. Change each to wrap the string and drop the trailing executor:
`Loop(ctx, "some cmd", loopCtx, opts, mock)` → `Loop(ctx, ShellCommand(mock, "some cmd"), loopCtx, opts)`.
For any test that passed a `CommandFn` directly, just drop the trailing `mock` argument.

- [ ] **Step 5: Run loop tests (expect PASS)**

Run: `go test ./internal/loop/ -v`
Expected: PASS (including `TestShellCommandRunsViaExecutor`).

- [ ] **Step 6: Add `runLoopCommand` to `helpers.go`**

In `internal/cli/helpers.go` add (it needs `os`, `loop`, `config` — `os`, `loop`, `config` are already imported):

```go
// runLoopCommand resolves config + meta dir and runs command across all
// projects with opts, exiting non-zero if any project failed. Shared body of
// the exec/git/npm loop commands.
func runLoopCommand(command loop.CommandFn, opts loop.Options) error {
	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}
	cfg, err := resolveConfig()
	if err != nil {
		return err
	}
	results, err := loop.Loop(runCtx(), command, loop.Context{
		Config:  cfg.Config,
		MetaDir: metaDir,
	}, opts)
	if err != nil {
		return err
	}
	if loop.GetExitCode(results) != 0 {
		os.Exit(1)
	}
	return nil
}
```

- [ ] **Step 7: Collapse the simple loop commands**

For these five, the entire `runX` body becomes (showing `git_status`; the others differ only by the command string):

```go
func runGitStatus(cmd *cobra.Command, _ []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	return runLoopCommand(loop.ShellCommand(newShellExecutor(), "git status --short --branch"), opts)
}
```

| File | function | command string |
|------|----------|----------------|
| `git_status.go` | `runGitStatus` | `git status --short --branch` |
| `git_pull.go` | `runGitPull` | `git pull` |
| `git_push.go` | `runGitPush` | `git push` |
| `npm_install.go` | `runNpmInstall` | `npm install` |
| `npm_install.go` | `runNpmCi` | `npm ci` |

After editing, remove the now-unused `"os"` import from each of these files (the `os.Exit` moved into `runLoopCommand`). Keep the `loop` import.

- [ ] **Step 8: Collapse the commands that build their command string**

`git_branch.go` `runGitBranch`:

```go
func runGitBranch(cmd *cobra.Command, args []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	deleteFlag, _ := cmd.Flags().GetBool("delete")
	allFlag, _ := cmd.Flags().GetBool("all")

	command := "git branch"
	switch {
	case len(args) == 0 && allFlag:
		command = "git branch -a"
	case len(args) == 0:
		command = "git branch"
	case deleteFlag:
		command = fmt.Sprintf("git branch -d %s", args[0])
	default:
		command = fmt.Sprintf("git branch %s", args[0])
	}

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
```

`git_checkout.go` `runGitCheckout`:

```go
func runGitCheckout(cmd *cobra.Command, args []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	command := fmt.Sprintf("git checkout %s", args[0])
	if create, _ := cmd.Flags().GetBool("create"); create {
		command = fmt.Sprintf("git checkout -b %s", args[0])
	}

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
```

`git_commit.go` `runGitCommit` (note the forced-sequential override is preserved):

```go
func runGitCommit(cmd *cobra.Command, _ []string) error {
	message, _ := cmd.Flags().GetString("message")

	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	opts.Parallel = false // git commit is always sequential

	escapedMessage := strings.ReplaceAll(message, `"`, `\"`)
	command := fmt.Sprintf(`git commit -m "%s"`, escapedMessage)

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
```

These keep their `fmt`/`strings` imports and drop `os`.

- [ ] **Step 9: Collapse the commands with a preamble**

`exec.go` `runExec`:

```go
func runExec(cmd *cobra.Command, args []string) error {
	command := args[0]

	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	output.Info(fmt.Sprintf("Executing: %s", output.Bold(command)))

	return runLoopCommand(loop.ShellCommand(newShellExecutor(), command), opts)
}
```

`npm_run.go` `runNpmRun` (keeps the `--if-present` `CommandFn`; drops `var command any` and `os`):

```go
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
			return exec.Execute(ctx, fmt.Sprintf("npm run %s", script), executor.Options{Cwd: absoluteDir})
		}
	} else {
		command = loop.ShellCommand(exec, fmt.Sprintf("npm run %s", script))
	}

	return runLoopCommand(command, opts)
}
```

(`hasNpmScript` is unchanged. Keep imports `context`, `encoding/json`, `fmt`, `os`, `path/filepath`, `executor`, `loop`, `output`, `cobra` — `os` is still used by `hasNpmScript` via `os.ReadFile`, so keep it.)

- [ ] **Step 10: Update `run.go` to the new `Loop` API (stays bespoke)**

`run.go` resolves config to look up the command and builds merged filter options, so it does not use `runLoopCommand`. Only update its `loop.Loop` call:

```go
	results, err := loop.Loop(runCtx(), loop.ShellCommand(newShellExecutor(), commandDef.Cmd), loop.Context{
		Config:  configResult.Config,
		MetaDir: metaDir,
	}, loopOpts)
```

(Drop the trailing `newShellExecutor()` argument that used to be the `exec` param; it now lives inside `ShellCommand`.) Leave the rest of `run.go` unchanged.

- [ ] **Step 11: Add a `runLoopCommand` test (success path)**

Add to `internal/cli/helpers_test.go` (reuses `captureOut`/`chdir`; `os.Exit` failure path is intentionally not unit-tested, consistent with the rest of the CLI):

```go
func TestRunLoopCommandRunsAcrossProjects(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA","b":"urlB"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b"), 0o755))
	config.SetOverlayFiles(nil)
	_ = captureOut(t)
	chdir(t, dir)

	var mu sync.Mutex
	ran := map[string]bool{}
	command := loop.CommandFn(func(_ context.Context, _ , projectPath string) (*executor.Result, error) {
		mu.Lock()
		ran[projectPath] = true
		mu.Unlock()
		return &executor.Result{ExitCode: 0}, nil
	})

	err := runLoopCommand(command, loop.Options{})
	require.NoError(t, err)
	assert.True(t, ran["a"])
	assert.True(t, ran["b"])
}
```

Add imports as needed (`context`, `sync`, `path/filepath`, `loop`, `executor`, `config`).

- [ ] **Step 12: Build + test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS. `grep -rn 'command any\|executeCommand' internal/loop/` returns nothing.

```bash
git add internal/loop/loop.go internal/loop/loop_test.go internal/cli/
git commit -m "refactor(loop,cli): type-safe Loop with CommandFn and shared runLoopCommand"
```

---

## Task 4: Route `ssh` through the `Executor` interface; delete `ExecuteSync`

**Files:**
- Modify: `internal/ssh/ssh.go` (`AddHostKey`, `EnsureSSHHostsKnown` take an `executor.Executor`)
- Modify: `internal/cli/git_clone.go`, `internal/cli/git_update.go` (pass an executor)
- Modify: `internal/executor/executor.go` (delete `ExecuteSync`)
- Modify: `internal/executor/executor_test.go` (delete `TestExecuteSync`)
- Test: `internal/ssh/ssh_test.go` (add a mock-executor test)

**Interfaces:**
- Consumes: `executor.Executor`, `executor.NewShellExecutor`.
- Produces:
  - `func AddHostKey(exec executor.Executor, host string) bool`
  - `func EnsureSSHHostsKnown(exec executor.Executor, urls []string) (added, failed []string)`

- [ ] **Step 1: Write the failing ssh test**

Add to `internal/ssh/ssh_test.go` (define a tiny mock if the file lacks one):

```go
type stubExecutor struct {
	exitCode int
	gotCmd   string
}

func (s *stubExecutor) Execute(_ context.Context, command string, _ executor.Options) (*executor.Result, error) {
	s.gotCmd = command
	return &executor.Result{ExitCode: s.exitCode}, nil
}

func TestAddHostKeyUsesExecutor(t *testing.T) {
	ok := &stubExecutor{exitCode: 0}
	assert.True(t, AddHostKey(ok, "example.com"))
	assert.Contains(t, ok.gotCmd, "ssh-keyscan")
	assert.Contains(t, ok.gotCmd, "example.com")

	bad := &stubExecutor{exitCode: 1}
	assert.False(t, AddHostKey(bad, "example.com"))
}
```

Add imports: `context`, `testing`, `github.com/daFish/gogo-meta/internal/executor`, testify `assert`.

- [ ] **Step 2: Run it (expect FAIL)**

Run: `go test ./internal/ssh/ -run TestAddHostKeyUsesExecutor -v`
Expected: FAIL — `AddHostKey` takes one argument, not two (compile error).

- [ ] **Step 3: Thread the executor through `ssh.go`**

In `internal/ssh/ssh.go`:
- Add `"context"` to imports.
- Change `AddHostKey`:

```go
func AddHostKey(exec executor.Executor, host string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	command := `ssh-keyscan -H "` + host + `" >> "` + knownHostsPath + `" 2>/dev/null`

	result, err := exec.Execute(context.Background(), command, executor.Options{Cwd: "."})
	if err != nil {
		return false
	}
	return result.ExitCode == 0
}
```

- Change `EnsureSSHHostsKnown` to take and forward the executor:

```go
func EnsureSSHHostsKnown(exec executor.Executor, urls []string) (added, failed []string) {
	hosts := ExtractUniqueSSHHosts(urls)
	for _, host := range hosts {
		if !IsHostKnown(host) {
			output.Info("Adding SSH host key for " + host + "...")
			if AddHostKey(exec, host) {
				output.Success("Added host key for " + host)
				added = append(added, host)
			} else {
				output.Error("Failed to add host key for " + host)
				failed = append(failed, host)
			}
		}
	}
	return added, failed
}
```

- [ ] **Step 4: Update the three call sites**

In `internal/cli/git_clone.go` (two calls) and `internal/cli/git_update.go` (one call), change
`ssh.EnsureSSHHostsKnown(urls)` → `ssh.EnsureSSHHostsKnown(newShellExecutor(), urls)`
and `ssh.EnsureSSHHostsKnown([]string{url})` → `ssh.EnsureSSHHostsKnown(newShellExecutor(), []string{url})`.
(`newShellExecutor()` returns an `executor.Executor` and is already defined in `helpers.go`.)

- [ ] **Step 5: Delete `ExecuteSync`**

In `internal/executor/executor.go`, delete the `ExecuteSync` function. In `internal/executor/executor_test.go`, delete `TestExecuteSync`. Confirm no remaining callers:

Run: `grep -rn 'ExecuteSync' internal/ cmd/ --include="*.go"`
Expected: no output.

- [ ] **Step 6: Build + test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS (`ssh` now has a mockable test).

```bash
git add internal/ssh/ssh.go internal/ssh/ssh_test.go internal/cli/git_clone.go internal/cli/git_update.go internal/executor/executor.go internal/executor/executor_test.go
git commit -m "refactor(ssh,executor): inject Executor into ssh and drop ExecuteSync"
```

---

## Task 5: Fix parallel/sequential result discarding

**Files:**
- Modify: `internal/loop/loop.go` (`runSequential`, `runParallel`)
- Test: `internal/loop/loop_test.go`

**Interfaces:**
- Consumes: the typed `command CommandFn` runners from Task 3.
- Produces: both runners record a failed `Result` on a per-project execution error instead of discarding the batch.

- [ ] **Step 1: Write the failing test**

Add to `internal/loop/loop_test.go`. Use a `CommandFn` that errors for one project and succeeds for the others:

```go
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
	assert.False(t, byDir["b"].Success, "the erroring project is marked failed, not discarded")
}
```

Add imports `bytes`, `errors` if missing. (Note: `Loop` signature is the Task-3 form — no trailing executor.)

- [ ] **Step 2: Run it (expect FAIL)**

Run: `go test ./internal/loop/ -run TestParallelKeepsResultsWhenOneErrors -v`
Expected: FAIL — current `runParallel` returns `nil, firstErr` on the error, so `Loop` returns `nil` results and the length assertion fails.

- [ ] **Step 3: Fix `runParallel`**

Replace the body of `runParallel` so a per-item error becomes a failed `Result` and cancels siblings, and the function never discards results:

```go
func runParallel(ctx context.Context, command CommandFn, directories []string, loopCtx Context, opts Options) ([]Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				projectPath := directories[idx]
				absoluteDir := filepath.Join(loopCtx.MetaDir, projectPath)

				start := time.Now()
				result, err := command(ctx, absoluteDir, projectPath)
				duration := time.Since(start)

				if err != nil {
					cancel()
					results[idx] = Result{
						Directory: projectPath,
						Result:    executor.Result{ExitCode: 1, Stderr: err.Error()},
						Success:   false,
						Duration:  duration,
					}
					continue
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

	for _, r := range results {
		output.Header(r.Directory)
		output.CommandOutput(r.Result.Stdout, r.Result.Stderr)
	}

	return results, nil
}
```

(Remove the now-unused `mu`/`firstErr` declarations.)

- [ ] **Step 4: Align `runSequential`**

In `runSequential`, replace the `if err != nil { return nil, err }` block with a failed-result record that continues:

```go
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
```

Ensure `executor` is imported in `loop.go` (it is, via `executor.Result`).

- [ ] **Step 5: Run tests (expect PASS)**

Run: `go test ./internal/loop/ -v`
Expected: PASS, including `TestParallelKeepsResultsWhenOneErrors`.

- [ ] **Step 6: Build + test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS.

```bash
git add internal/loop/loop.go internal/loop/loop_test.go
git commit -m "fix(loop): record per-project failures instead of discarding results"
```

---

## Final verification (after all tasks)

- [ ] `make all` (clean, lint, test-coverage, build) green; `ssh` coverage up.
- [ ] `grep -rn 'ExecuteStreaming\|StreamingOptions\|ExecuteSync\|SuppressOutput\|command any' internal/` → nothing; `grep -rn 'executeCommand\|panic(' internal/loop/` → nothing.
- [ ] Manual: `gogo git status`, `gogo exec "echo hi"`, `gogo exec "false" --parallel` (summary still reports the repos that succeeded), `gogo npm run build --if-present`, and a clone against an unknown SSH host all behave as before.
- [ ] Tag the completed stage: `git tag -a go-rewrite-cleanup -m "Go rewrite cleanup: dead code, typed Loop, ssh DI, parallel fix"`.
```
