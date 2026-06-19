# Shell-Free Built-ins + Signal Handling

**Date:** 2026-06-18
**Branch:** `refactor/go-rewrite` (work on a feature branch off it)

## Goal

Close two security findings in the Go rewrite before it is pushed:

1. **CRITICAL — shell command injection / RCE-on-clone.** Built-in git/ssh/npm
   commands run via `/bin/sh -c <string>` with interpolated data. A cloned repo's
   `.gogo` project URLs (and the SSH hosts parsed from them) flow into
   `git clone "%s"` / `ssh-keyscan -H "host"`; double-quote wrapping does not stop
   `$(...)`/backticks. So `gogo git clone <attacker-repo>` and `gogo git update`
   execute attacker-controlled shell. Fix: run all built-in commands as **argv
   slices, with no shell**.
2. **IMPORTANT — no signal handling.** Every command uses `context.Background()`;
   Ctrl-C orphans in-flight git/npm children (the `Setpgid` cleans up nothing).
   Fix: wire SIGINT/SIGTERM → context cancellation → process-group kill.

`/bin/sh -c` remains ONLY for `gogo exec` and `gogo run`, where executing a shell
string is the command's contract.

## Parity note

This **intentionally diverges** from JS `main`, which uses `spawn(..., {shell:true})`
and carries the same vulnerability. Behavior is identical for valid inputs; only
malicious inputs stop executing. Do not "restore parity" by reverting argv → shell
for built-ins.

## Out of scope

- The three Minors from the review (executor `TrimSpace` of output, the 533-line
  `config.go` split, `%w` error wrapping) — separate cleanup.
- `gogo run` executing an untrusted repo's `.gogo` `commands` strings: that is
  shell-by-contract (like running `make` in an untrusted checkout). Documented
  residual risk, not addressed here.

## Design

### 1. Executor: add an argv method

`internal/executor/executor.go`. Extend the interface:

```go
type Executor interface {
    Execute(ctx context.Context, command string, opts Options) (*Result, error)               // /bin/sh -c — exec/run only
    ExecuteArgs(ctx context.Context, name string, args []string, opts Options) (*Result, error) // argv — all built-ins
}
```

Factor the shared run mechanics (timeout context, `Setpgid`, stdout/stderr capture,
exit-code/timeout handling, process-group kill) into one private helper that takes a
prepared `*exec.Cmd`:

- `Execute` builds `exec.CommandContext(ctx, "/bin/sh", "-c", command)` (unchanged).
- `ExecuteArgs` builds `exec.CommandContext(ctx, name, args...)`.
- Both set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and
  `cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }`
  so context cancellation (SIGINT or timeout) kills the **whole child process group**,
  not just the direct child. Set `cmd.WaitDelay` to a small grace (e.g. 2s) so
  `cmd.Wait` returns promptly after cancel.

`ExecuteArgs` preserves the existing `Result` semantics (exit code, captured
stdout/stderr, `TimedOut`). The current output `TrimSpace` behavior is kept as-is
(out of scope to change here).

The three test mocks gain an `ExecuteArgs` method:
- `internal/loop/loop_test.go` `mockExecutor`
- `internal/cli/migrate_test.go` `fakeExecutor`
- `internal/ssh/ssh_test.go` `stubExecutor`

### 2. Signal-aware cancellation

- `cmd/gogo/main.go`:

```go
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

- `internal/cli/helpers.go`: `runCtx()` is replaced by reading the command's context.
  Cobra propagates the `ExecuteContext` ctx to every command via `cmd.Context()`.
  Change `runLoopCommand` and the loop commands to pass `cmd.Context()` instead of
  `runCtx()`; replace every `ctx := context.Background()` in `git_clone.go`,
  `git_update.go`, `project_create.go`, `project_import.go`, `npm_link.go` with
  `ctx := cmd.Context()`. (`migrate` and `ssh` receive their ctx as a parameter from
  the caller — thread `cmd.Context()` down to them.)

Net: Ctrl-C cancels the shared context → the executor's `cmd.Cancel` kills each
running child's process group → no orphans.

### 3. Loop built-ins → argv

`internal/loop/loop.go`: add an argv analog of `ShellCommand`:

```go
// ArgsCommand adapts an argv invocation into a CommandFn run via exec (no shell).
func ArgsCommand(exec executor.Executor, name string, args ...string) CommandFn {
    return func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) {
        return exec.ExecuteArgs(ctx, name, args, executor.Options{Cwd: absoluteDir})
    }
}
```

Convert the built-in loop commands from `ShellCommand` to `ArgsCommand`:

| Command | argv |
|---------|------|
| `git_status` | `"git","status","--short","--branch"` |
| `git_pull` | `"git","pull"` |
| `git_push` | `"git","push"` |
| `git_checkout` | `"git","checkout",[ "-b", ] branch` |
| `git_branch` | `"git","branch"` / `…,"-a"` / `…,"-d",name` / `…,name` |
| `git_commit` | `"git","commit","-m",message` (no escaping — message is one arg) |
| `npm_install` | `"npm","install"` |
| `npm_ci` | `"npm","ci"` |
| `npm_run` | `"npm","run",script` (both the `--if-present` CommandFn and the else branch) |

`exec` and `run` keep `ShellCommand` (shell is their contract). `git_branch`/
`git_checkout` build a `[]string` from flags instead of `fmt.Sprintf`; `git_commit`
drops the `strings.ReplaceAll` escaping entirely.

### 4. Direct (non-loop) callers → argv

Replace `exec.Execute(ctx, fmt.Sprintf(...))` with `ExecuteArgs`:

- `git_clone.go` (meta clone + child clones): `"git","clone",url,dir`.
- `git_update.go` (child clones): `"git","clone",url,dir`.
- `project_create.go`: `"git","init"` and `"git","remote","add","origin",url`.
- `project_import.go` (clone): `"git","clone",url,dir`.
- `migrate.go`: `"git","remote","get-url","origin"`.

URLs and paths become plain args — no quoting, no interpolation, no injection.

### 5. ssh-keyscan → argv + Go-side known_hosts append

`internal/ssh/ssh.go` `AddHostKey` currently runs
`ssh-keyscan -H "host" >> knownHostsPath 2>/dev/null` — shell redirection that argv
cannot express. Rewrite:

- `result, err := exec.ExecuteArgs(ctx, "ssh-keyscan", []string{"-H", host}, executor.Options{})`
- On `err == nil && result.ExitCode == 0 && result.Stdout != ""`: append
  `result.Stdout + "\n"` to `knownHostsPath` via `os.OpenFile(…, O_APPEND|O_CREATE|O_WRONLY, 0o600)`.
- Return success on a non-empty appended key; the old `2>/dev/null` becomes "ignore
  stderr / non-zero exit → false".

This removes the shell, the redirect, and the host-interpolation injection together.
`AddHostKey`/`EnsureSSHHostsKnown` keep their injected-`Executor` signatures from the
earlier cleanup.

## Files Changed

| File | Change |
|------|--------|
| `internal/executor/executor.go` | Add `ExecuteArgs`; factor shared `run` helper; add `cmd.Cancel` process-group kill + `WaitDelay`. |
| `internal/executor/executor_test.go` | Tests: `ExecuteArgs` runs argv & captures; an injection-attempt arg is treated literally (not executed). |
| `cmd/gogo/main.go` | `signal.NotifyContext` + `ExecuteContext(ctx)`. |
| `internal/cli/helpers.go` | Drop `runCtx()`; use `cmd.Context()`; thread ctx into `runLoopCommand`. |
| `internal/loop/loop.go` | Add `ArgsCommand`. |
| `internal/loop/loop_test.go` | `mockExecutor.ExecuteArgs`; `ArgsCommand` test. |
| `internal/cli/{git_status,git_pull,git_push,git_checkout,git_branch,git_commit,npm_install,npm_run}.go` | Built-ins → `ArgsCommand`; ctx from `cmd.Context()`. |
| `internal/cli/{git_clone,git_update,project_create,project_import}.go` | `ExecuteArgs`; ctx from `cmd.Context()`. |
| `internal/cli/migrate.go` | `ExecuteArgs` for `git remote get-url`; ctx threaded. |
| `internal/cli/migrate_test.go` | `fakeExecutor.ExecuteArgs`. |
| `internal/ssh/ssh.go` | `ssh-keyscan` via `ExecuteArgs` + Go-side known_hosts append. |
| `internal/ssh/ssh_test.go` | `stubExecutor.ExecuteArgs`; test that captured stdout is appended to a temp known_hosts. |

## Testing

- `ExecuteArgs` runs an argv command and captures output; an arg like
  `"; touch pwned"` or `"$(touch pwned)"` is passed literally and does **not** create
  the file (the core regression guard).
- `ArgsCommand` runs the argv via the executor across a project dir.
- ssh: with a stub `Executor` returning a fake key on stdout, `AddHostKey` appends it
  to a `t.TempDir()` known_hosts and returns true; exit-non-zero/empty → false, no append.
- Mock executors implement `ExecuteArgs` and assert built-ins pass argv (not a shell
  string) — e.g. `git_commit` test asserts the message reaches the command as a single
  arg, unescaped.
- Signal/cancel: if a deterministic test is impractical, document a manual check
  (long `gogo exec sleep 30` across repos, Ctrl-C, confirm children die).
- Each phase: `make lint` (0 issues) and `make test` green.

## Verification

1. `make all` green.
2. `grep -rn '/bin/sh' internal/` → only the executor's `Execute` (shell) path; no
   built-in builds a shell string.
3. `grep -rn 'fmt.Sprintf(\`git \|fmt.Sprintf("npm ' internal/cli/` → nothing (no
   interpolated shell command strings remain for built-ins).
4. Manual RCE check: a `.gogo` with project URL `$(touch /tmp/pwned)` + `gogo git
   update` does **not** create `/tmp/pwned` (clone fails cleanly instead).
5. Manual signal check: Ctrl-C during a multi-repo run leaves no orphaned git/npm
   processes.
