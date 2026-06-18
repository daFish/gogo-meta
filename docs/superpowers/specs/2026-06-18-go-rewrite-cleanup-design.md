# Go Rewrite Cleanup

**Date:** 2026-06-18
**Branch:** `refactor/go-rewrite`

## Goal

Remove port residue and type-unsafety surfaced by a code review of the Go rewrite,
without changing `gogo`'s observable behavior — with one exception: a genuine bug fix
in parallel mode. The JS implementation on `main` remains the behavioral reference;
none of these changes diverge from it (the deletions remove code JS also does not use).

## Context

The Go rewrite faithfully ported TypeScript, including dead code and a `string | CommandFn`
union expressed through `any` + a runtime `panic`. Verified facts driving this cleanup:

- `ExecuteStreaming`/`StreamingOptions`/`OnStdout`/`OnStderr` have **zero production callers**
  in Go (only `executor_test.go`). The JS twin `executeStreaming` is **also** unused in `src/`
  — `loop.ts` uses buffered `execute`. Deleting it in Go is parity-safe.
- `loop.Options.SuppressOutput` is **never set `true`** — not in production, not in tests
  (tests silence output via `output.Writer` override). Permanently false.
- `loop.Loop` takes `command any` and `executeCommand` `panic`s on an unexpected type. The
  only non-string caller is `npm_run` (its `--if-present` `CommandFn`); the other 10 callers
  pass strings.
- 10 command `RunE` bodies repeat an identical 5-step sequence; `git_status.go` and
  `git_pull.go` differ only by the command string.
- `ssh.AddHostKey` calls the concrete `executor.ExecuteSync` directly, bypassing the
  `Executor` interface (so `ssh` cannot be mocked). `ExecuteSync` is otherwise a verbatim
  third copy of the `Execute` shell/`Setpgid`/timeout block; its only caller is `ssh`.
- `runParallel` discards all successful results when a single `executeCommand` returns an
  error, and does not cancel the shared context for sibling workers.

## Out of scope

- Live/interleaved parallel output (parallel keeps buffered print-in-order).
- Cleaning the dead `executeStreaming` on the JS `main` side (separate, different branch).
- Any change to command semantics or output strings.

## Design

### Phase 1 — Dead-code purge

**A. Delete the streaming subsystem.** Remove from `internal/executor/executor.go`:
`ExecuteStreaming`, `StreamingOptions` (and its `OnStdout`/`OnStderr` fields). Remove
`TestExecuteStreaming` from `executor_test.go`. No production code references them.

**B. Delete `SuppressOutput`.** Remove the field from `loop.Options` and the four
`if !opts.SuppressOutput { … }` guards in `internal/loop/loop.go`, so the guarded output
(`Header`, `CommandOutput`, `Summary`) always runs — which is the only behavior that ever
occurred. No caller sets it, so no call site changes.

### Phase 2 — Type-safe `Loop`

**C.** In `internal/loop/loop.go`:
- Change `Loop`'s signature from `command any` to `command CommandFn`, and have `Loop` invoke
  `command(ctx, absoluteDir, projectPath)` directly.
- Add `func ShellCommand(exec executor.Executor, s string) CommandFn` that returns a closure
  capturing the executor and the shell string:
  `return func(ctx context.Context, absoluteDir, _ string) (*executor.Result, error) { return exec.Execute(ctx, s, executor.Options{Cwd: absoluteDir}) }`.
- Delete `executeCommand` and its `panic` (the type switch is gone).

Call-site changes (11):
- The 10 string callers change `loop.Loop(ctx, "git pull", …, exec)` to
  `loop.Loop(ctx, loop.ShellCommand(exec, "git pull"), …, exec)` — in practice routed through
  the Phase 3 helper, so most are rewritten there.
- `npm_run` already builds a `CommandFn`; it drops the `var command any` and passes the
  `CommandFn` (or `loop.ShellCommand(exec, …)` in its else branch) directly.

### Phase 3 — De-duplicate the command layer

**D.** Add to `internal/cli/helpers.go`:

```go
// runLoopCommand resolves config + loop options and runs command across all
// projects, exiting non-zero if any project failed. It is the shared body of
// the exec/run/git/npm loop commands.
func runLoopCommand(cmd *cobra.Command, command loop.CommandFn) error
```

It performs: `requireMetaDir` → `resolveConfig` → `resolveLoopOptions` → `loop.Loop(...)` →
`if loop.GetExitCode(results) != 0 { os.Exit(1) }`. The 10 loop commands
(`exec, run, git_status, git_pull, git_push, git_checkout, git_commit, git_branch,
npm_install, npm_run`) reduce their `RunE` to building the command and calling the helper.
For the plain ones (`git_status`, `git_pull`, `git_push`, `npm_install`) the body becomes a
single `return runLoopCommand(cmd, loop.ShellCommand(newShellExecutor(), "git pull"))`.
Commands that print a pre-amble (`exec` "Executing: …", `npm_run` "Running …") keep that
line before the helper call.

Note: `npm_install` runs two loop commands (install, then ci) in sequence — it calls the
helper twice; preserve that.

### Phase 4 — `ssh` through the `Executor` interface

**E.** Thread an executor through `internal/ssh/ssh.go`:
- `func AddHostKey(exec executor.Executor, host string) bool` — calls
  `exec.Execute(context.Background(), command, executor.Options{Cwd: "."})` instead of
  `executor.ExecuteSync`.
- `func EnsureSSHHostsKnown(exec executor.Executor, urls []string) (added, failed []string)`
  — passes `exec` down to `AddHostKey`.
- Callers `git_clone.go` (two sites) and `git_update.go` pass `newShellExecutor()` /
  `executor.NewShellExecutor()`.
- With `ssh` migrated, `executor.ExecuteSync` has **no callers** → delete `ExecuteSync` and
  `TestExecuteSync`. This removes the third duplicate of the shell/`Setpgid`/timeout block.

Behavior is preserved: Go's `Execute` already blocks until the process exits (`cmd.Run()`),
so using it with `context.Background()` is equivalent to the old `ExecuteSync`.

### Phase 5 — Parallel result-discarding fix

**F.** Make both runners follow one rule: **a per-project execution error is recorded as a
failed `Result`, never discards other results.** A `command(...)` error is a rare
infrastructure failure (the executor itself failing); a non-zero exit is already a normal
`Result`, not an error.

- In `runParallel`: derive `ctx, cancel := context.WithCancel(ctx); defer cancel()`. When an
  item's `command(...)` returns an error, write a failed `Result` at that index
  (`Success: false`, `Result: executor.Result{ExitCode: 1, Stderr: err.Error()}`) and
  `cancel()` so siblings stop early — do **not** `return` from the goroutine discarding the
  slice. Remove the `firstErr`/abort path.
- In `runSequential`: replace `if err != nil { return nil, err }` with the same "record a
  failed `Result`, continue" behavior, so the two modes agree.
- `Loop` then always returns `(results, nil)` for per-project failures; the existing
  `GetExitCode`/`Summary` already turn failed results into a non-zero exit. The `error` return
  remains only for genuine setup failures (currently none).

## Files Changed

| File | Change |
|------|--------|
| `internal/executor/executor.go` | Delete `ExecuteStreaming`, `StreamingOptions` (A); delete `ExecuteSync` after ssh migrates (E). |
| `internal/executor/executor_test.go` | Delete `TestExecuteStreaming` (A) and `TestExecuteSync` (E). |
| `internal/loop/loop.go` | Delete `SuppressOutput` + guards (B); `Loop` takes `CommandFn`, add `ShellCommand`, delete `executeCommand`+panic (C); fix `runParallel` discard + ctx cancel (F). |
| `internal/loop/loop_test.go` | Update `Loop` call sites to `CommandFn`/`ShellCommand`; add parallel-with-one-failure test (F); add `ShellCommand` test (C). |
| `internal/cli/helpers.go` | Add `runLoopCommand` (D). |
| `internal/cli/{exec,run,git_status,git_pull,git_push,git_checkout,git_commit,git_branch,npm_install,npm_run}.go` | Reduce `RunE` to build-command + `runLoopCommand` (C/D). |
| `internal/cli/helpers_test.go` | Add a `runLoopCommand` test (D). |
| `internal/ssh/ssh.go` | `AddHostKey`/`EnsureSSHHostsKnown` take `executor.Executor` (E). |
| `internal/ssh/ssh_test.go` | Add an `AddHostKey`/`EnsureSSHHostsKnown` test with a mock `Executor` (E). |
| `internal/cli/{git_clone,git_update}.go` | Pass executor to `ssh.EnsureSSHHostsKnown` (E). |

## Testing

- Each phase keeps `make lint` (0 issues) and `make test` green.
- C: a test that `ShellCommand(exec, "echo hi")` runs the string via the executor; `Loop`
  compiles only with `CommandFn`.
- D: a `runLoopCommand` test (mock executor, temp `.gogo`) asserting it runs across projects
  and the non-zero-exit path is reachable (extract the exit decision so it is testable without
  `os.Exit`, or assert via results — see plan).
- E: an `ssh` test with a mock `Executor` asserting `AddHostKey` returns true/false on
  exit 0/non-0 and that `EnsureSSHHostsKnown` calls it per unknown host.
- F: a parallel run where one project's command errors at the infrastructure level — assert the
  other projects' successful results are still present and the batch is not discarded.

## Verification

1. `make all` green (lint, test-coverage, build). `ssh` coverage rises (now mockable).
2. `grep -rn 'ExecuteStreaming\|StreamingOptions\|ExecuteSync\|SuppressOutput\|command any\|panic(' internal/` returns nothing for the removed symbols.
3. Manual: `gogo git status`, `gogo exec "echo hi" --parallel`, `gogo npm run build --if-present`,
   and a clone against an unknown SSH host all behave as before.
4. `gogo exec --parallel` where one repo's command fails: the summary still reports the other
   repos' successes (the bug fix).
