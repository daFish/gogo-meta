# Shell-Free Built-ins + Signal Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate shell command injection (run all built-in git/ssh/npm commands as argv, no `/bin/sh`) and add SIGINT/SIGTERM cancellation that kills child process groups.

**Architecture:** Add `ExecuteArgs(ctx, name, args, opts)` to the `Executor` interface (argv, no shell) alongside `Execute` (shell, kept for `exec`/`run`); both share one run helper that sets a `cmd.Cancel` process-group kill. Wire `signal.NotifyContext` → `ExecuteContext` → `cmd.Context()` so cancellation propagates. Convert loop built-ins to a new `loop.ArgsCommand`, direct callers to `ExecuteArgs`, and rewrite `ssh-keyscan` to argv + Go-side `known_hosts` append.

**Tech Stack:** Go 1.26, cobra, testify, golangci-lint v2.

## Global Constraints

- Built-in git/ssh/npm commands MUST run via argv (`ExecuteArgs`), never `/bin/sh -c`. `/bin/sh -c` survives ONLY in `Execute`, used only by `gogo exec`/`run`.
- No observable behavior change for valid inputs. Intentional divergence from JS (`spawn shell:true`): malicious inputs stop executing. Do not revert argv→shell for built-ins.
- Cancellation: SIGINT/SIGTERM → root context cancel → `cmd.Cancel` kills the child process group (the `Setpgid` is already set).
- Keep `Result` semantics (exit code, captured trimmed stdout/stderr, `TimedOut`). The output `TrimSpace` is out of scope — leave it.
- Idiomatic Go; `internal/` only; errcheck/staticcheck/gocritic/gosec; `0o600` for known_hosts. `make lint` (0 issues) and `make test` green before each commit.
- **Commit style: headline only — NO `Issue:` trailer, NO `Co-Authored-By:` trailer.** `git commit -m "type(scope): subject"`. Never push.
- Task order: Task 1 (executor) first — Tasks 2-4 depend on `ExecuteArgs`.

---

## Task 1: Executor — `ExecuteArgs` + process-group kill

**Files:**
- Modify: `internal/executor/executor.go`
- Modify: `internal/executor/executor_test.go`
- Modify: `internal/loop/loop_test.go`, `internal/cli/migrate_test.go`, `internal/ssh/ssh_test.go` (add `ExecuteArgs` to mocks so the interface still compiles)

**Interfaces:**
- Produces:
  - `Executor.ExecuteArgs(ctx context.Context, name string, args []string, opts Options) (*Result, error)`
  - `ShellExecutor` implements both `Execute` and `ExecuteArgs` via a private `run`.
  - Cancellation kills the child process group.

- [ ] **Step 1: Write the injection regression test (RED)**

Add to `internal/executor/executor_test.go`:

```go
func TestExecuteArgsTreatsArgsLiterally(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	e := NewShellExecutor()

	// If args were passed through a shell, $(...) / ; would execute and create marker.
	res, err := e.ExecuteArgs(context.Background(), "echo", []string{"$(touch " + marker + ")", "; touch " + marker}, Options{Cwd: dir})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "args must be literal, not shell-evaluated")
	assert.Contains(t, res.Stdout, "$(touch") // echoed verbatim
}

func TestExecuteArgsCapturesExit(t *testing.T) {
	e := NewShellExecutor()
	res, err := e.ExecuteArgs(context.Background(), "false", nil, Options{Cwd: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, 1, res.ExitCode)
}
```

Ensure imports `os`, `path/filepath`.

- [ ] **Step 2: Run it (expect FAIL)**

Run: `go test ./internal/executor/ -run TestExecuteArgs -v`
Expected: FAIL — `ExecuteArgs` undefined.

- [ ] **Step 3: Refactor `executor.go` — shared `run`, add `ExecuteArgs`, interface, group-kill**

Replace the interface and the `Execute` method in `internal/executor/executor.go` with:

```go
// Executor runs commands. Execute uses /bin/sh -c (for user shell commands);
// ExecuteArgs runs an argv directly with no shell (for built-in commands).
type Executor interface {
	Execute(ctx context.Context, command string, opts Options) (*Result, error)
	ExecuteArgs(ctx context.Context, name string, args []string, opts Options) (*Result, error)
}

// Execute runs a command string via /bin/sh -c. Use only where running an
// arbitrary shell command is the contract (gogo exec / run).
func (e *ShellExecutor) Execute(ctx context.Context, command string, opts Options) (*Result, error) {
	return e.run(ctx, "/bin/sh", []string{"-c", command}, opts)
}

// ExecuteArgs runs name with args directly — no shell, so args cannot be
// interpreted as shell syntax.
func (e *ShellExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts Options) (*Result, error) {
	return e.run(ctx, name, args, opts)
}

func (e *ShellExecutor) run(ctx context.Context, name string, args []string, opts Options) (*Result, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = opts.Cwd
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}

	// Run each command in its own process group and, on context cancellation
	// (Ctrl-C or timeout), kill the whole group so no children are orphaned.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	timedOut := ctx.Err() == context.DeadlineExceeded

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	if timedOut {
		exitCode = 124
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		TimedOut: timedOut,
	}, nil
}
```

Add `"errors"` to the imports.

- [ ] **Step 4: Run it (expect PASS)**

Run: `go test ./internal/executor/ -run TestExecuteArgs -v`
Expected: PASS.

- [ ] **Step 5: Add `ExecuteArgs` to the three mocks**

Each mock currently implements only `Execute`. Add an `ExecuteArgs` that joins the argv into a command string and reuses the existing behavior, so they still satisfy `Executor`:

`internal/loop/loop_test.go` (after `mockExecutor.Execute`):

```go
func (m *mockExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts executor.Options) (*executor.Result, error) {
	return m.Execute(ctx, strings.TrimSpace(name+" "+strings.Join(args, " ")), opts)
}
```

`internal/cli/migrate_test.go` (after `fakeExecutor.Execute`):

```go
func (f *fakeExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts executor.Options) (*executor.Result, error) {
	return f.Execute(ctx, strings.TrimSpace(name+" "+strings.Join(args, " ")), opts)
}
```

`internal/ssh/ssh_test.go` (after `stubExecutor.Execute`):

```go
func (s *stubExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts executor.Options) (*executor.Result, error) {
	return s.Execute(ctx, strings.TrimSpace(name+" "+strings.Join(args, " ")), opts)
}
```

Add `"strings"` to each test file's imports if missing.

- [ ] **Step 6: Lint + full test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS.

```bash
git add internal/executor/ internal/loop/loop_test.go internal/cli/migrate_test.go internal/ssh/ssh_test.go
git commit -m "feat(executor): add shell-free ExecuteArgs and process-group cancellation"
```

---

## Task 2: Signal wiring + loop built-ins → argv

**Files:**
- Modify: `cmd/gogo/main.go`
- Modify: `internal/cli/helpers.go` (`runLoopCommand` takes a ctx; drop `runCtx`)
- Modify: `internal/loop/loop.go` (add `ArgsCommand`)
- Modify: `internal/cli/{exec,git_status,git_pull,git_push,git_checkout,git_branch,git_commit,npm_install,npm_run,run}.go`
- Test: `internal/loop/loop_test.go` (add `ArgsCommand` test)

**Interfaces:**
- Consumes: `executor.ExecuteArgs` (Task 1).
- Produces:
  - `loop.ArgsCommand(exec executor.Executor, name string, args ...string) CommandFn`
  - `runLoopCommand(ctx context.Context, command loop.CommandFn, opts loop.Options) error`
  - `main` runs via `rootCmd.ExecuteContext(ctx)` with a signal-bound ctx; built-in loop commands run argv and observe cancellation.

- [ ] **Step 1: Add `ArgsCommand` test (RED)**

Add to `internal/loop/loop_test.go`:

```go
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
```

(`newMockExecutor` keys by `opts.Cwd`; the `ExecuteArgs` mock from Task 1 forwards to `Execute`, so the `/tmp` key matches.)

- [ ] **Step 2: Run it (expect FAIL)**

Run: `go test ./internal/loop/ -run TestArgsCommandRunsViaExecutor -v`
Expected: FAIL — `ArgsCommand` undefined.

- [ ] **Step 3: Add `ArgsCommand` to `loop.go`**

In `internal/loop/loop.go`, after `ShellCommand`:

```go
// ArgsCommand adapts an argv invocation into a CommandFn run via exec with no
// shell, so arguments cannot be interpreted as shell syntax.
func ArgsCommand(exec executor.Executor, name string, args ...string) CommandFn {
	return func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
		return exec.ExecuteArgs(ctx, name, args, executor.Options{Cwd: absoluteDir})
	}
}
```

- [ ] **Step 4: Run it (expect PASS)**

Run: `go test ./internal/loop/ -run TestArgsCommandRunsViaExecutor -v`
Expected: PASS.

- [ ] **Step 5: Signal-bind in `main.go`**

Replace `cmd/gogo/main.go`:

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/daFish/gogo-meta/internal/cli"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/daFish/gogo-meta/internal/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd := cli.NewRootCommand(version.Info())
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Thread ctx through `runLoopCommand` (helpers.go)**

In `internal/cli/helpers.go`, change `runLoopCommand` to accept a ctx and delete `runCtx`:

```go
func runLoopCommand(ctx context.Context, command loop.CommandFn, opts loop.Options) error {
	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}
	cfg, err := resolveConfig()
	if err != nil {
		return err
	}
	results, err := loop.Loop(ctx, command, loop.Context{
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

Delete the `runCtx()` function. Add `"context"` to imports if missing.

- [ ] **Step 7: Convert the loop commands to argv + pass `cmd.Context()`**

The simple ones become a single `return`:

```go
// git_status.go
return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", "status", "--short", "--branch"), opts)
// git_pull.go
return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", "pull"), opts)
// git_push.go
return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", "push"), opts)
// npm_install.go runNpmInstall
return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "npm", "install"), opts)
// npm_install.go runNpmCi
return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "npm", "ci"), opts)
```

`git_checkout.go` `runGitCheckout` — build args, no `fmt.Sprintf`:

```go
func runGitCheckout(cmd *cobra.Command, args []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	gitArgs := []string{"checkout", args[0]}
	if create, _ := cmd.Flags().GetBool("create"); create {
		gitArgs = []string{"checkout", "-b", args[0]}
	}

	return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", gitArgs...), opts)
}
```

(Drop the now-unused `"fmt"` import from `git_checkout.go`.)

`git_branch.go` `runGitBranch`:

```go
func runGitBranch(cmd *cobra.Command, args []string) error {
	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}

	deleteFlag, _ := cmd.Flags().GetBool("delete")
	allFlag, _ := cmd.Flags().GetBool("all")

	gitArgs := []string{"branch"}
	switch {
	case len(args) == 0 && allFlag:
		gitArgs = []string{"branch", "-a"}
	case len(args) == 0:
		gitArgs = []string{"branch"}
	case deleteFlag:
		gitArgs = []string{"branch", "-d", args[0]}
	default:
		gitArgs = []string{"branch", args[0]}
	}

	return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", gitArgs...), opts)
}
```

(Drop `"fmt"` from `git_branch.go`.)

`git_commit.go` `runGitCommit` — message is one arg, no escaping:

```go
func runGitCommit(cmd *cobra.Command, _ []string) error {
	message, _ := cmd.Flags().GetString("message")

	opts, err := resolveLoopOptions(cmd)
	if err != nil {
		return err
	}
	opts.Parallel = false // git commit is always sequential

	return runLoopCommand(cmd.Context(), loop.ArgsCommand(newShellExecutor(), "git", "commit", "-m", message), opts)
}
```

(Drop `"fmt"` and `"strings"` from `git_commit.go`.)

`npm_run.go` `runNpmRun` — both branches argv:

```go
	exec := newShellExecutor()
	var command loop.CommandFn
	if ifPresent, _ := cmd.Flags().GetBool("if-present"); ifPresent {
		command = func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
			if !hasNpmScript(absoluteDir, script) {
				return &executor.Result{ExitCode: 0, Stdout: fmt.Sprintf("Script %q not found, skipping", script)}, nil
			}
			return exec.ExecuteArgs(ctx, "npm", []string{"run", script}, executor.Options{Cwd: absoluteDir})
		}
	} else {
		command = loop.ArgsCommand(exec, "npm", "run", script)
	}

	return runLoopCommand(cmd.Context(), command, opts)
```

(`fmt` still used by the skip message and the preamble; keep it.)

`run.go` — bespoke; change its `loop.Loop` call to use `cmd.Context()` and `ArgsCommand` (its `commandDef.Cmd` is a user-defined shell command, so it KEEPS `ShellCommand`):

```go
	results, err := loop.Loop(cmd.Context(), loop.ShellCommand(newShellExecutor(), commandDef.Cmd), loop.Context{
		Config:  configResult.Config,
		MetaDir: metaDir,
	}, loopOpts)
```

(`run` executes a `.gogo`-defined shell command — shell is its contract, so `ShellCommand` stays. Only the ctx source changes.)

`exec.go` — keeps `ShellCommand` (user shell command), change ctx source:

```go
	return runLoopCommand(cmd.Context(), loop.ShellCommand(newShellExecutor(), command), opts)
```

- [ ] **Step 8: Lint + full test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS. `grep -rn 'fmt.Sprintf(\`git ' internal/cli/` → nothing.

```bash
git add cmd/gogo/main.go internal/cli/ internal/loop/
git commit -m "feat(cli,loop): run built-in git/npm commands via argv and honor signals"
```

---

## Task 3: Direct (non-loop) callers → argv

**Files:**
- Modify: `internal/cli/git_clone.go`, `internal/cli/git_update.go`, `internal/cli/project_create.go`, `internal/cli/project_import.go`, `internal/cli/npm_link.go`, `internal/cli/migrate.go`

**Interfaces:**
- Consumes: `executor.ExecuteArgs` (Task 1).
- Produces: every direct git/npm invocation runs argv; ctx comes from `cmd.Context()`.

- [ ] **Step 1: Convert each call site**

Replace each `exec.Execute(ctx, fmt.Sprintf(...))` / static-string `Execute` with `ExecuteArgs`, and replace `ctx := context.Background()` with `ctx := cmd.Context()`:

| File:line | Before | After |
|-----------|--------|-------|
| `git_clone.go:61` | `ctx := context.Background()` | `ctx := cmd.Context()` |
| `git_clone.go:63` | `exec.Execute(ctx, fmt.Sprintf(\`git clone "%s" "%s"\`, url, repoName), …)` | `exec.ExecuteArgs(ctx, "git", []string{"clone", url, repoName}, …)` |
| `git_clone.go:125` | `exec.Execute(ctx, fmt.Sprintf(\`git clone "%s" "%s"\`, projectURL, filepath.Base(projectPath)), …)` | `exec.ExecuteArgs(ctx, "git", []string{"clone", projectURL, filepath.Base(projectPath)}, …)` |
| `git_update.go:89` | `ctx := context.Background()` | `ctx := cmd.Context()` |
| `git_update.go:104` | `exec.Execute(ctx, fmt.Sprintf(\`git clone "%s" "%s"\`, m.url, filepath.Base(m.path)), …)` | `exec.ExecuteArgs(ctx, "git", []string{"clone", m.url, filepath.Base(m.path)}, …)` |
| `project_create.go:45` | `ctx := context.Background()` | `ctx := cmd.Context()` |
| `project_create.go:47` | `exec.Execute(ctx, "git init", …)` | `exec.ExecuteArgs(ctx, "git", []string{"init"}, …)` |
| `project_create.go:55` | `exec.Execute(ctx, fmt.Sprintf(\`git remote add origin "%s"\`, url), …)` | `exec.ExecuteArgs(ctx, "git", []string{"remote", "add", "origin", url}, …)` |
| `project_import.go:42` | `ctx := context.Background()` | `ctx := cmd.Context()` |
| `project_import.go:47` | `exec.Execute(ctx, "git remote get-url origin", …)` | `exec.ExecuteArgs(ctx, "git", []string{"remote", "get-url", "origin"}, …)` |
| `project_import.go:110` | `exec.Execute(ctx, fmt.Sprintf(\`git clone "%s" "%s"\`, url, filepath.Base(folder)), …)` | `exec.ExecuteArgs(ctx, "git", []string{"clone", url, filepath.Base(folder)}, …)` |
| `npm_link.go:84` | `ctx := context.Background()` | `ctx := cmd.Context()` |
| `npm_link.go:125` | `exec.Execute(ctx, "npm link", …)` | `exec.ExecuteArgs(ctx, "npm", []string{"link"}, …)` |

For `migrate.go`: `getRemoteURL(ex executor.Executor, dir string)` uses `context.Background()` and is shared by the command path. Change its `Execute` to argv and accept a ctx:

```go
func getRemoteURL(ctx context.Context, ex executor.Executor, dir string) string {
	res, err := ex.ExecuteArgs(ctx, "git", []string{"remote", "get-url", "origin"}, executor.Options{Cwd: dir})
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}
```

Thread a `ctx` parameter into `runMigrate(ctx context.Context, ex executor.Executor, cwd string, dryRun bool)` and its internal `getRemoteURL` calls; the cobra action passes `cmd.Context()`:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			...
			code, err := runMigrate(cmd.Context(), executor.NewShellExecutor(), cwd, dryRun)
			...
		},
```

Update `internal/cli/migrate_test.go` calls to `runMigrate(...)` to pass `context.Background()` as the new first argument.

After edits, drop now-unused imports (`fmt` may still be used for output messages — keep where so; remove only if a file no longer references it). Each file: confirm it still compiles.

- [ ] **Step 2: Build + test**

Run: `make lint && make test`
Expected: 0 issues; all PASS.

- [ ] **Step 3: Manual injection check**

Run:
```bash
go build -o /tmp/gogo ./cmd/gogo
mkdir -p /tmp/rce && cd /tmp/rce && /tmp/gogo init
printf '{"projects":{"x":"$(touch /tmp/PWNED)"}}' > .gogo
/tmp/gogo git update; ls /tmp/PWNED 2>/dev/null && echo "VULNERABLE" || echo "safe"
```
Expected: `safe` — the clone fails (bad URL) and `/tmp/PWNED` is never created. Clean up `/tmp/rce`, `/tmp/gogo`.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): run direct git/npm invocations via argv"
```

---

## Task 4: ssh-keyscan → argv + Go-side known_hosts append

**Files:**
- Modify: `internal/ssh/ssh.go`
- Test: `internal/ssh/ssh_test.go`

**Interfaces:**
- Consumes: `executor.ExecuteArgs` (Task 1).
- Produces: `AddHostKey(exec, host)` runs `ssh-keyscan -H <host>` via argv and appends the captured key to `known_hosts` in Go — no shell, no redirection.

- [ ] **Step 1: Write the test (RED)**

Add to `internal/ssh/ssh_test.go`. The stub returns a fake key on stdout; assert it lands in a temp known_hosts. Override the home dir via `t.Setenv("HOME", dir)` so `os.UserHomeDir()` resolves into the temp dir:

```go
func TestAddHostKeyAppendsScannedKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))

	stub := &stubExecutor{exitCode: 0, stdout: "example.com ssh-rsa AAAAKEY"}
	ok := AddHostKey(stub, "example.com")
	require.True(t, ok)

	content, err := os.ReadFile(filepath.Join(home, ".ssh", "known_hosts"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "example.com ssh-rsa AAAAKEY")
}

func TestAddHostKeyFailsOnNonZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stub := &stubExecutor{exitCode: 1}
	assert.False(t, AddHostKey(stub, "example.com"))
}
```

Ensure `stubExecutor` carries a `stdout` field returned by its `Execute`/`ExecuteArgs` (extend it if it only had `exitCode`/`gotCmd`). Add imports `os`, `path/filepath`.

- [ ] **Step 2: Run it (expect FAIL)**

Run: `go test ./internal/ssh/ -run TestAddHostKey -v`
Expected: FAIL — current `AddHostKey` shells out with `>>` and the stub's stdout is never appended.

- [ ] **Step 3: Rewrite `AddHostKey`**

In `internal/ssh/ssh.go`, replace `AddHostKey`:

```go
// AddHostKey scans host's SSH key with ssh-keyscan (argv, no shell) and appends
// it to known_hosts. Returns true if a key was obtained and written.
func AddHostKey(exec executor.Executor, host string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	result, err := exec.ExecuteArgs(context.Background(), "ssh-keyscan", []string{"-H", host}, executor.Options{})
	if err != nil || result.ExitCode != 0 || result.Stdout == "" {
		return false
	}

	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(result.Stdout + "\n"); err != nil {
		return false
	}
	return true
}
```

(`context`, `os`, `filepath`, `executor` are imported already; keep them.)

- [ ] **Step 4: Run it (expect PASS)**

Run: `go test ./internal/ssh/ -run TestAddHostKey -v`
Expected: PASS.

- [ ] **Step 5: Lint + full test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS. `grep -rn '/bin/sh\|>> ' internal/ssh/` → nothing.

```bash
git add internal/ssh/
git commit -m "feat(ssh): scan host keys via argv and append in Go, no shell"
```

---

## Final verification (after all tasks)

- [ ] `make all` green.
- [ ] `grep -rn '/bin/sh' internal/` → only `executor.go`'s `Execute` path.
- [ ] `grep -rn 'fmt.Sprintf(\`git \|fmt.Sprintf("npm ' internal/cli/` → nothing (no interpolated shell strings for built-ins).
- [ ] Manual RCE check (Task 3 Step 3) → `safe`.
- [ ] Manual signal check: `gogo exec "sleep 30"` across ≥2 repos, Ctrl-C, then `pgrep -f "sleep 30"` → nothing (no orphans).
- [ ] Tag the stage: `git tag -a shell-safety -m "shell-free built-ins + signal handling"`.
```
