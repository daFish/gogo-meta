# `gogo update` — implementation design

| | |
| --- | --- |
| Status | Phases 0 and 1 implemented 2026-09-08; Decisions 1, 11 and 12 settled; Phase 2 proposed |
| Target release | 3.2.0 (`feat`) after the Phase 0 fixes ship as patches |
| Baseline | v3.1.1, `main` at 93f1224 |
| Scope | New top-level command that converges the working copy on the config in one run: move renamed projects, clone missing ones, fast-forward the rest. Plus four prerequisite fixes in existing commands. |

All `file:line` references are against the baseline above.

**Settled by the maintainer (2026-09-08).** Both confirm the recommendation that was already written
here; they are recorded so the plan carries decisions rather than proposals.

| | Decision | Effect |
| --- | --- | --- |
| Decision 1 | "Recursively" means **(a) every configured project**, not descent into child meta repos | Option (b) stays fully designed below but moves to Phase 2.1; nothing in Phase 0/1 depends on it |
| Open question 11 | `gogo migrate` **keeps exit 1** for missing/ambiguous projects | Phase 0.3 only aligns `--dry-run` with the real run; the real run's contract is unchanged, so `TestMigrateMissingExitsNonZero` stays as-is |

The remaining eleven open questions in §10 are still open.

---

## 1. Problem statement

Today three commands each own one slice of "make the working copy match `.gogo`", and each one's failure message sends the user to another command:

1. `gogo validate` finds a missing project directory and prints
   `directory missing — run 'gogo migrate' if it moved, or 'gogo git update' to clone`
   (`internal/cli/validate.go:96` `missingDirectoryHint`, emitted at `validate.go:164`), exit 1.
2. `gogo migrate` applies every move it can, then for each project it could not find prints
   `<path> not found in working copy — run 'gogo git update' to clone` (`internal/cli/migrate.go:220`)
   and returns exit code 1 (`migrate.go:228-230`). A run that moved one repo and lacked one repo
   prints `✓ libs/api moved from oldapi` *and* exits 1. The exit code says "failed" for a command
   that did its job; the message is a dead end because it points at a third command.
3. `gogo migrate --dry-run` returns 0 (`migrate.go:223-226`) on the same working-copy state where
   the real run returns 1 — the preview lies about the exit code.
4. `gogo git update <dir>` is not a real form: `newGitUpdateCmd` sets no `Args:`
   (`internal/cli/git_update.go:15-24`), so cobra accepts and silently discards positional
   arguments and the command clones every missing project. The targeted form is `--include-only`.
5. `--parallel` / `--concurrency` are registered on `git update` (`git_update.go:22`
   `addParallelFlags(cmd)`) and documented (`README.md:412,421-422`), but `runGitUpdate` never reads
   them — the clone loop at `git_update.go:96-130` is strictly sequential. The flags are inert.
6. `gogo git pull` against a missing project directory yields an empty failure: the loop runs
   `git pull` with `Cwd` set to a non-existent directory (`internal/loop/loop.go:107`, via
   `git_pull.go:24`); `exec.Cmd.Run` fails with a `chdir` error that is not an `*exec.ExitError`, so
   `executor.run` maps it to `ExitCode: 1` with empty `Stderr` (`internal/executor/executor.go:94-102`);
   the loop prints nothing for it and the summary lists the project as failed with no reason.
7. Nothing in the codebase traverses a child's own `.gogo`. `gogo git clone` clones the meta repo
   and its direct children exactly once (`internal/cli/git_clone.go:80-148`);
   `discover.FindGitRepos` explicitly does not descend into discovered repositories
   (`internal/discover/discover.go:46-49`).
8. The two hint strings in (1) and (2) have been unchanged since v3.0.0 (commit a243633).

The user's ask — "a new command `gogo update` which makes sure ALL projects are recursively up2date" — is
the convergent command that (1)–(3) keep pointing at but which does not exist.

### Goals

- One command, `gogo update`, after which (on exit 0) every selected project is present at its
  configured path and, where it is on a branch with an upstream, fast-forwarded to that upstream.
- Precise, documented exit-code contract; `--dry-run` previews that contract instead of contradicting it.
- Reuse the existing migrate, clone and loop machinery; no second implementation of any of them.
- Fix the four defects (3), (4), (5), (6) first, as independently shippable patches.

### Non-goals (initial scope)

- Descending into child meta repositories (see Decision 1, option b — designed here, deferred).
- Changing what `gogo git pull` does with a dirty tree or diverged branch (it stays a thin `git pull`).
- Submodule handling, remote pruning, branch switching, stashing.

---

## 2. Decisions

### Decision 1 — What does "recursively" mean?

**Option (a): across every configured project.** "Recursively" reads as "all of them, including the
ones under nested paths such as `libs/api`". `gogo update` walks the merged config once, exactly like
`gogo git update` and `gogo migrate` do. No new capability beyond composition.

**Option (b): descend into child meta repositories.** After a child is present and current, look for
a `.gogo` / `.gogo.yaml` / `.gogo.yml` at *its* root and run the same pipeline there, and so on. Given
fact 7 this is a substantial feature, not a flag. It would additionally require:

| Concern | What is needed |
| --- | --- |
| Detection | A `config.FindMetaFileIn(dir)` that looks only in `dir` — `FindMetaFileUp` (`config.go:382`) walks *up* and would find the parent's file. `requireOwnedConfig` (`config.go:367`) still applies per file. |
| Cycle detection | A visited set keyed by canonical remote URL (and the real path of the directory) along the *ancestor chain*, so `A → B → A` stops, while the same library appearing under two siblings is allowed once each. |
| Depth limit | `--max-depth <n>` (hard stop, default in the 5–10 range) as a bound independent of cycle detection, because a chain of distinct repos can be arbitrarily long and each level runs network operations. |
| Config merge semantics | None. A child's config is read on its own with its own `.gogo.local` (via `config.ReadMetaConfig(childDir, []string{})`, as `git_clone.go:86` does). The root's `-f` overlays name root paths and groups and must **not** be applied to children — `config.SetOverlayFiles` (`config.go:145`) is process-global and has to be bypassed. |
| Path namespacing | Output lines prefixed with the child's path (`libs/api/vendor/x`). Filters: root `--include-only` / `--group` select root projects only; whether a filter can reach into a child (`libs/api/vendor/x`) is undefined and must be decided. `.gitignore` and `.git/info/exclude` maintenance stays per meta repo (each child owns its own files). |
| Ordering / parallelism | A child's own children can only be read after the child is present and current, so levels are sequential; the parallel pool applies within a level. |
| Security | Cloning an untrusted meta repo would transitively clone whatever its children's configs name. `IsSafeProjectPath` (`config.go:215`) keeps each level inside its own directory, but the surface grows with depth. Recursion must be opt-in (`--recursive`), never the default, and `gogo git clone` would need the same opt-in for parity. |

**Decided: (a), every configured project.** Confirmed by the maintainer on 2026-09-08. It is the
plain reading, it resolves facts 1–6, and it is what every other gogo command means by "all
projects". Option (b) is a follow-up with its own
design (Phase 2.1 below); nothing in this design precludes it, and the pipeline is written as
`runUpdate(ctx, ex, metaDir, …)` so it can later be invoked per level.

### Decision 2 — Naming and collision with `gogo git update` / `gogo git pull`

| Option | What happens to the existing commands | Back-compat |
| --- | --- | --- |
| **Composition (recommended)** | Both stay, unchanged in meaning. `gogo git update` remains the clone-only primitive, `gogo git pull` the plain `git pull` primitive. `gogo update` is built *from* the same functions (`cloneMissing`, `planMigration`) rather than from the commands. | None broken. Scripts using `gogo git update` / `gogo git pull` keep working. |
| Alias | `gogo git update` becomes an alias of `gogo update --no-migrate --no-pull`. | Would drag the new command's summary format and the `syncLocalExcludes` timing into an existing command for no gain; rejected. |
| Deprecation | Mark `gogo git update` deprecated, remove in 4.0. | Premature: the primitive is still useful in scripts that must not pull. Revisit after `gogo update` has been in a release. |

Two behavior changes to `gogo git update` do ship, as **fixes**, not as deprecation:

- `Args: cobra.NoArgs` (fact 4). `gogo git update libs/web` will error with cobra's
  `unknown command "libs/web" for "gogo git update"` instead of cloning everything. This turns a silent
  misbehavior into an error; the commit body must call it out. The same defect exists on
  `git status`, `git pull`, `git push`, `git commit`, `npm install`, `npm link` (none set `Args:`) — see
  Open question 8.
- `--parallel` / `--concurrency` honored (fact 5).

Help text must make the two `update`s distinguishable at a glance:

- `gogo git update` — `Short: "Clone missing repositories"` (unchanged).
- `gogo update` — `Short: "Bring the working copy up to date: move renamed projects, clone missing ones, fast-forward the rest"`.

### Decision 3 — Does `gogo update` also perform migrate's moves?

**Yes.** The pipeline is **migrate moves → clone missing → pull existing**. Any reading that omits the
move phase leaves a rename bouncing the user to `gogo migrate` — the exact ping-pong in facts 1–2.

Consequences, all accepted:

- **Conflict semantics.** `runMigrate` refuses to move anything when any configured path is occupied
  by a repository with a different (or no) `origin` URL (`migrate.go:149-155`, `167-176`). `gogo update`
  inherits this for its **move phase**: no move is applied if any selected project is in conflict.
  Unlike `gogo migrate`, the **clone and pull phases still run** for the projects that are not
  affected (see §3.4 for why that is safe); the conflicting projects and the un-applied moves are
  reported as failed and the run exits 1.
- **Its own `--dry-run`**, covering all three phases and returning the exit code the real run would
  (fixing the fact-3 defect by construction, and the migrate one in Phase 0.3).
- **`--no-migrate`** opt-out, because moving directories and editing `.gitignore` is the one phase
  that rewrites the user's tree. A project whose repository is found at another path is then
  reported as failed (`found at <old> — run without --no-migrate to move it`) and is **not** cloned,
  since cloning would produce a second checkout.
- The move path keeps every side effect of `migrate.go:183-214`: `os.Rename`, `pruneEmptyParents`,
  `config.RemoveFromGitignore(from)` + `config.AddToGitignore(to)` for non-local projects,
  then `ensureLocalConfigIgnored` and `syncLocalExcludes` once any move happened. This is achieved by
  extracting that code into functions both commands call, not by copying it.

### Decision 4 — `--no-clone` exit contract, and the converse

Two states look alike ("project not at its configured path") but differ in kind:

| State | Under the opt-out flag | Class | Why |
| --- | --- | --- | --- |
| **Absent** — URL found nowhere in the working copy | `--no-clone` | **skipped, exit 0** | The user said "do not clone". Partial checkouts are a legitimate steady state; a permanent exit 1 would make the flag useless in scripts (they would have to ignore the exit code and lose real pull failures). Every skipped project is still printed and counted in the summary. |
| **Misplaced** — URL found at another path | `--no-migrate` | **failed, exit 1** | There *is* a checkout, in the wrong place. Ignoring it hides a real discrepancy; cloning would duplicate it. |

Alternative for the first row: keep exit 1 ("the working copy is not converged"). Rejected for the
reason above; `--strict` (Open question 3) can add it later without changing the default.

**Converse.** Clone-only-no-pull is `gogo update --no-pull`. It is **not** identical to today's
`gogo git update`: it also moves. `gogo update --no-migrate --no-pull` is the closest equivalent and
differs only in output format. All three phases have an opt-out flag (`--no-migrate`, `--no-clone`,
`--no-pull`) so the surface is symmetric; passing all three is rejected with an error
("nothing to do: all phases disabled").

### Decision 5 — Pull policy

`gogo update` never creates merge commits, never rebases, never stashes, never switches branches.
Per present project it performs a read-only inspection, a fetch, and at most one fast-forward:

| Situation | Behavior | Class |
| --- | --- | --- |
| Detached HEAD | Not pulled: `skipped: detached HEAD` | skipped |
| Branch has no upstream | Not pulled: `skipped: no upstream branch` | skipped |
| Fetch fails (network, auth, upstream ref gone) | `fetch failed: <first stderr line>` | failed |
| Behind 0, ahead 0 | `up to date` | ok |
| Behind 0, ahead N | `up to date (ahead N, not pushed)` | ok |
| Behind M, ahead 0 | `git merge --ff-only @{u}`; on success `updated (M commit(s))` | ok |
| Behind M, ahead N (diverged) | Not attempted: `diverged from upstream (ahead N, behind M) — merge or rebase manually` | failed |
| Dirty working tree | **Not a gate.** The fast-forward is attempted; git itself refuses when local or untracked changes overlap the incoming tree, and that refusal is reported as `fast-forward refused: <first stderr line>`. When the tree is dirty the message appends `(working tree has local changes)`. | ok or failed, decided by git |

Rationale for the dirty-tree rule: options were (i) skip dirty trees, (ii) attempt `--ff-only` and let
git guard, (iii) `--autostash`. (i) makes the command useless for the daily "I have WIP everywhere"
case and hides that the repo is behind; (iii) can leave a half-applied stash on pop conflicts.
(ii) is exactly what `git pull --ff-only` does for a human: a fast-forward is a checkout of the new
tree, it succeeds when the WIP does not overlap and is refused when it does. It never produces
conflict markers.

Mechanics use `git fetch` + `git rev-list --left-right --count HEAD...@{u}` + `git merge --ff-only @{u}`
rather than a single `git pull`: no dependence on the user's `pull.rebase` / `pull.ff` settings, the
diverged case is detected *before* touching the tree, and no localized git message has to be parsed
(the executor cannot set `LC_ALL=C` without replacing the whole environment, `executor.go:68-70`).

Skipped states exit 0: a detached HEAD or a local-only branch is usually deliberate and the tool
cannot fix it. The convergence contract is stated with that carve-out (§3.4).

---

## 3. Command surface

### 3.1 Synopsis

```
gogo update [--dry-run] [--no-migrate] [--no-clone] [--no-pull]
            [--parallel] [--concurrency <n>]
            [--group <names>] [--include-only <dirs>] [--exclude-only <dirs>]
            [--include-pattern <re>] [--exclude-pattern <re>]
```

`Args: cobra.NoArgs`. Registered in `NewRootCommand` (`root.go:39-48`) between `newMigrateCmd()` and
`newGitCmd()`.

### 3.2 Flags

| Flag | Source | Default | Meaning |
| --- | --- | --- | --- |
| `--dry-run` | local | false | Print the plan for all three phases; change nothing on disk, run no `git fetch`/`clone`/`merge`. Exit code follows §3.4 evaluated on the plan. |
| `--no-migrate` | local | false | Do not move directories. A misplaced project is reported as failed and not cloned. |
| `--no-clone` | local | false | Do not clone. An absent project is reported as skipped. |
| `--no-pull` | local | false | Do not fetch or fast-forward present projects. |
| `--parallel` | `addParallelFlags` (`helpers.go:39-42`) | false | Run the clone phase and the pull phase through the bounded pool. The move phase is always sequential. |
| `--concurrency <n>` | `addParallelFlags` | 0 → `loop.DefaultConcurrency` (4) | Pool size. |
| `--group`, `--include-only`, `--exclude-only`, `--include-pattern`, `--exclude-pattern` | `addFilterFlags` (`helpers.go:31-37`), resolved with `resolveFilterOptionsWithConfig(cmd, &cfg)` (`helpers.go:78-111`) | — | Select projects. Groups intersect with the other filters (`filter.Apply` `GroupOnly` stage). Overlay announcement happens once (commit 3c698cb). |
| `-f, --file` | root persistent (`root.go:30`) | — | Overlays, as everywhere. |

The root also declares the filter and parallel flags as persistent (`root.go:31-37`); the local
declarations shadow them, and `getStringFlag` / `getBoolFlag` / `getIntFlag` (`helpers.go:44-67`)
fall back to the inherited ones. Flag placement (`gogo --group x update` vs `gogo update --group x`)
must behave as `TestGroupFlagPositions` (`internal/cli/group_test.go:90`) specifies; the new command
is added to that test's list.

### 3.3 Pipeline

```
runUpdate(ctx, ex, cwd, opts) (exitCode int, err error)

  0. metaDir     := config.GetMetaDir(cwd)                       // "" → notAGogoRepoMsg
     result      := config.ReadMetaConfig(cwd, nil); printOverlayInfo(result)
     syncLocalExcludes(metaDir, result.LocalProjects)           // filter-independent, always, first (mirrors git_update.go:37-42)
     selected    := filter.Apply(sorted(cfg.Projects keys), opts.Filter)
     if empty → output.Warning("No projects match the specified filters"); return 0

  1. plan := planMigration(ctx, ex, metaDir, cfg, selected)      // extracted from migrate.go:130-165
       buckets: present  (dir exists, origin == URL)
                moves    (dir absent, URL found at exactly one other path)
                missing  (dir absent, URL found nowhere)
                ambiguous(dir absent, URL found at ≥2 paths)
                conflicts(dir exists, origin != URL or no origin)

  2. MOVE phase   (skipped when --no-migrate; printed only when --dry-run)
       conflicts non-empty → every conflict and every move is reported failed; no rename happens
       else applyMoves(metaDir, plan.moves, localProjects)      // extracted from migrate.go:183-214
            → moved projects join `present`
       --no-migrate → every move reported failed ("found at X — run without --no-migrate to move it")

  3. CLONE phase  (skipped when --no-clone → each missing project reported skipped)
       ambiguous → failed, never cloned
       warnUnverifiedSSHHosts(urls)
       cloneMissing(ctx, ex, metaDir, targets, opts.Parallel, opts.Concurrency)   // shared with git update
       freshly cloned projects are NOT pulled (they are current by construction)

  4. PULL phase   (skipped when --no-pull)
       for each project in `present` (including the ones moved in step 2), via loop.RunEach:
         pullProject(ctx, ex, dir) → Decision 5 table

  5. output.Info("moved a, cloned b, updated c, up to date d, skipped e, failed f")
     output.Summary(SummaryData{Success, Failed, Total, FailedProjects})
     return 1 if f > 0 else 0
```

The cobra `RunE` wraps it exactly like `newMigrateCmd` (`migrate.go:35-48`): build `updateOptions`
from flags, call `runUpdate(cmd.Context(), executor.NewShellExecutor(), cwd, opts)`, `os.Exit(code)`
when non-zero. `runUpdate` takes the `executor.Executor` interface so tests inject a scripted one.

Under `--parallel`, per-project status lines for a phase are printed after that phase's pool
drains, in input order — the same choice `runParallel` makes (`loop.go:186-191`) to avoid interleaved
output. Sequential mode prints as it goes.

### 3.4 Per-project outcome classes and exit-code contract

Every selected project ends the run in exactly one class:

| Class | Outcomes |
| --- | --- |
| **ok** | `moved from X`, `cloned`, `updated (N commit(s))`, `up to date`, `up to date (ahead N, not pushed)`, `would move/clone/pull` (dry run) |
| **skipped** | `not cloned (--no-clone)`, `detached HEAD`, `no upstream branch` |
| **failed** | `occupied by a different repository (found X)`, `multiple working-copy directories share its repository URL — resolve manually`, `not moved: migration aborted by a conflict`, `found at X — run without --no-migrate to move it`, clone error, `fetch failed: …`, `diverged …`, `fast-forward refused: …`, timeout (`executor.Result.TimedOut`) |

**Exit rule:** `1` if any project is *failed*, `0` otherwise. Config, filter and "not a gogo repo"
errors return an `error` from `RunE` (cobra prints it, exit 1), as today.

**Convergence guarantee on exit 0:** every selected project is at its configured path with the
configured origin, and every one of them that is on a branch with an upstream is at or ahead of that
upstream — except projects explicitly skipped by `--no-clone`, `--no-pull`, `--no-migrate`, or listed
as skipped for a detached HEAD / no upstream.

**Why running clone and pull after a conflict is safe.** `planMigration` puts each project in
exactly one bucket (`migrate.go:149-164`). A *missing* project's URL was found nowhere, so its clone
target cannot be the destination of an un-applied move (that project would be in `moves`, not
`missing`). *Present* projects have the configured origin, so pulling them is correct regardless of
conflicts elsewhere. The only projects that must not be touched are the conflicts themselves and the
pending moves, and both are marked failed without any action.

**Scenario matrix.** Rows are working-copy states for one selected project; columns are flags
(each column assumes the others at default). Cells give the reported outcome and the exit code
contributed by that project.

| # | State of the project | default | `--dry-run` | `--no-clone` | `--no-migrate` | `--no-pull` |
| --- | --- | --- | --- | --- | --- | --- |
| S1 | present, origin matches, up to date | up to date / 0 | would pull / 0 | up to date / 0 | up to date / 0 | (pull phase off) / 0 |
| S2 | present, behind, fast-forward possible | updated / 0 | would pull / 0 | updated / 0 | updated / 0 | (pull phase off) / 0 |
| S3 | absent (URL found nowhere) | cloned / 0 | would clone / 0 | **skipped / 0** | cloned / 0 | cloned / 0 |
| S3b | absent, clone fails (network, auth, bad URL) | failed / 1 | would clone / 0 | skipped / 0 | failed / 1 | failed / 1 |
| S4 | misplaced (URL found at one other path) | moved, then pulled / 0 | would move, would pull / 0 | moved, then pulled / 0 | **failed / 1** (not cloned) | moved / 0 |
| S5 | configured path occupied by a repo with a different or no origin | failed / 1; **no move applied for any project**; clones and pulls of others proceed | failed / 1 | failed / 1 | failed / 1 | failed / 1 |
| S6 | absent, URL found at two or more paths | failed / 1 (not cloned) | failed / 1 | failed / 1 | failed / 1 | failed / 1 |
| S7 | present, detached HEAD | skipped / 0 | would skip / 0 | skipped / 0 | skipped / 0 | (off) / 0 |
| S8 | present, no upstream | skipped / 0 | would skip / 0 | skipped / 0 | skipped / 0 | (off) / 0 |
| S9 | present, diverged | failed / 1 | would pull / 0 (cannot know without fetching — documented limitation) | failed / 1 | failed / 1 | (off) / 0 |
| S10 | present, dirty and the incoming change overlaps | failed / 1 (`fast-forward refused`) | would pull (working tree has local changes) / 0 | failed / 1 | failed / 1 | (off) / 0 |
| S11 | present, dirty, no overlap | updated / 0 | would pull (working tree has local changes) / 0 | updated / 0 | updated / 0 | (off) / 0 |
| S12 | no project matches the filters | warning, 0 | warning, 0 | warning, 0 | warning, 0 | warning, 0 |
| S13 | not inside a gogo repository | error, 1 | error, 1 | error, 1 | error, 1 | error, 1 |
| S14 | `--no-migrate --no-clone --no-pull` given together | error "nothing to do: all phases disabled", 1 | same | same | same | same |

Rules the matrix follows: (a) `--dry-run` returns 1 exactly when the *plan* already contains a
failure (S5, S6, and S4 combined with `--no-migrate`); it cannot predict pull or clone failures and
says so in its last line. (b) Skipped never contributes to exit 1. (c) The exit code is the same
whether `--parallel` is set or not.

**Comparison with the fixed `gogo migrate` (Phase 0.3):** `gogo migrate` keeps exit 1 for
missing/ambiguous projects (its existing contract, codified by `TestMigrateMissingExitsNonZero`);
`gogo migrate --dry-run` will return the same code the real run would for the same state. The
alternative — make both return 0 for missing, since missing is not migrate's job — was
considered and **rejected** by the maintainer on 2026-09-08 (former Open question 11): it is a
behavior change for scripts that depend on the current exit code.

### 3.5 Output example

```
$ gogo update --parallel
ℹ Using local overlay config: .gogo.local
ℹ Checking 4 projects...
✓ libs/api moved from oldapi
ℹ Cloning 1 missing project...
✓ libs/web cloned
ℹ Updating 3 projects...
✓ libs/api updated (3 commits)
✓ tools up to date
✗ services/auth diverged from upstream (ahead 1, behind 4) — merge or rebase manually

ℹ moved 1, cloned 1, updated 1, up to date 1, skipped 0, failed 1
⚠ 3/4 projects succeeded, 1 failed

Failed projects:
  ✗ services/auth
```

```
$ gogo update --dry-run
ℹ Would move oldapi → libs/api
ℹ Would clone libs/web
ℹ Would pull libs/api, tools (services/auth: detached HEAD, would skip)
ℹ Dry run: 1 move, 1 clone, 2 pulls pending; clone and pull failures cannot be predicted
```

All lines go through the existing `output` primitives (`output.ProjectStatus`, `Info`, `Warning`,
`Summary`), which already sanitize untrusted text (`output.go:22-33`).

---

## 4. Architecture

### 4.1 New package `internal/gitstate`

Read-only inspection and the fast-forward primitive, on top of `executor.Executor`. Everything the
pull policy needs, nothing else; fully unit-testable with a mocked executor.

```go
package gitstate

// Status is what `git status --porcelain=v2 --branch` reports about a working copy.
type Status struct {
    Head     string // commit id; "" for an unborn branch ("(initial)")
    Branch   string // "" when detached
    Detached bool
    Upstream string // "" when the branch has no upstream
    Dirty    bool   // any changed, renamed, unmerged or untracked entry
}

func Inspect(ctx context.Context, ex executor.Executor, dir string) (Status, error)
func ParsePorcelainV2(out string) (Status, error)          // pure; the part that gets table tests
func Fetch(ctx context.Context, ex executor.Executor, dir string) (*executor.Result, error)        // git fetch
func AheadBehind(ctx context.Context, ex executor.Executor, dir string) (ahead, behind int, err error) // git rev-list --left-right --count HEAD...@{u}
func FastForward(ctx context.Context, ex executor.Executor, dir string) (*executor.Result, error)  // git merge --ff-only @{u}
```

Porcelain v2 header lines consumed: `# branch.oid <sha>|(initial)`, `# branch.head <name>|(detached)`,
`# branch.upstream <ref>` (present only when set). Any line starting with `1 `, `2 `, `u ` or `? `
sets `Dirty`. `# branch.ab` is ignored (it is stale until fetch; `AheadBehind` is used instead).
`git fetch` with no arguments fetches the current branch's upstream remote, falling back to `origin`.

### 4.2 `internal/loop`: `RunEach` and the missing-directory guard

```go
// RunEach runs fn once per item and returns one Result per item, in input order.
// With parallel it uses a pool of at most concurrency workers (DefaultConcurrency when <= 0).
// It prints nothing. A non-nil error from fn cancels the context the remaining items see,
// matching Loop's behavior.
func RunEach(ctx context.Context, items []string, parallel bool, concurrency int,
    fn func(ctx context.Context, item string) (*executor.Result, error)) []Result
```

`runParallel` (`loop.go:131-194`) is reimplemented as `RunEach(...)` followed by its existing
print loop; `runSequential` stays as is because it streams `Header`/`CommandOutput` per project.
`cloneMissing` and the pull phase both use `RunEach`, so there is exactly one pool.

The fact-6 fix: `Loop` wraps `command` so that a project whose directory does not exist produces
`Result{Success: false, Result: executor.Result{ExitCode: 1, Stderr: MissingDirectoryMessage}}`
without invoking the command. `MissingDirectoryMessage` is a const in package `loop` (it cannot
import `cli`); its text is `directory missing — run 'gogo update' to clone it` from Phase 1 on, and
`… 'gogo git update' …` in Phase 0.4.

### 4.3 `internal/cli/clone.go` — shared clone phase

```go
type cloneTarget struct{ Path, URL string }
type cloneOutcome struct {
    Path string
    Err  string // "" means cloned
}

// cloneMissing validates every URL with giturl.Validate, creates the parent directory with
// os.MkdirAll(parent, 0o755), runs `git clone -- <url> <base>` in the parent, and classifies the
// result exactly as git_update.go:96-130 does today. Ordering of outcomes matches targets.
func cloneMissing(ctx context.Context, ex executor.Executor, metaDir string, targets []cloneTarget,
    parallel bool, concurrency int) []cloneOutcome
```

`runGitUpdate` becomes: `requireMetaDir` → `resolveConfig` → `syncLocalExcludes` → filter → find
missing → `warnUnverifiedSSHHosts` → `cloneMissing(..., getBoolFlag(cmd,"parallel"), getIntFlag(cmd,"concurrency"))`
→ print → `Summary` → `os.Exit(1)` on failure. Same messages as today.

### 4.4 `internal/cli/migrate.go` — extraction

```go
type migrationPlan struct {
    present   []string
    moves     []migrateMove
    missing   []string
    ambiguous []string
    conflicts []migrateConflict
}

func planMigration(ctx context.Context, ex executor.Executor, metaDir string, cfg config.MetaConfig,
    selected []string) (migrationPlan, error)                 // migrate.go:130-165, with `present` added

// applyMoves performs the renames and every side effect of migrate.go:183-214:
// MkdirAll(0o755), os.Rename, pruneEmptyParents, RemoveFromGitignore/AddToGitignore for non-local
// projects, then ensureLocalConfigIgnored and syncLocalExcludes once any move happened.
// It stops at the first filesystem error and returns it.
func applyMoves(metaDir string, moves []migrateMove, localProjects map[string]string) error
```

`runMigrate` calls both and keeps its own reporting and exit rules (plus the Phase 0.3 dry-run fix).
`planMigration` receives `selected` so `gogo update` can pass the filtered set while `gogo migrate`
passes every project (it has no filter flags today; adding them is out of scope).

### 4.5 `internal/cli/update.go`

```go
type updateOptions struct {
    Filter      filter.Options
    Parallel    bool
    Concurrency int
    DryRun      bool
    NoMigrate   bool
    NoClone     bool
    NoPull      bool
}

type projectOutcome struct {
    Path    string
    Class   outcomeClass // outcomeOK | outcomeSkipped | outcomeFailed
    Message string
}

func newUpdateCmd() *cobra.Command
func runUpdate(ctx context.Context, ex executor.Executor, cwd string, opts updateOptions) (int, error)
func pullProject(ctx context.Context, ex executor.Executor, dir string, dryRun bool) projectOutcome
func summarizeUpdate(outcomes []projectOutcome) (exitCode int)
```

`pullProject` is the Decision 5 table as a function of `gitstate` calls; `runUpdate` is the §3.3
pipeline. Both take the executor interface; neither calls `os.Getwd` or `os.Exit`.

### 4.6 `internal/cli/hints.go`

Home for the user-facing hint constants that several files share:
`missingDirectoryHint` (moved from `validate.go:96`), `notFoundHint` (the migrate line),
`cloneLaterHint` (`project_import.go:112`). Phase 1 changes their wording in one file.

### 4.7 Context, errors, style

- Every git invocation goes through `executor.Executor.ExecuteArgs` with `cmd.Context()`; the
  executor's per-command timeout (`executor.DefaultTimeout`, 5 min) applies to each clone, fetch and
  merge separately, and `Result.TimedOut` is reported as `timed out after 5m` (failed).
- No ignored return values: `applyMoves` returns the first error; `cloneMissing` and `pullProject`
  turn errors into failed outcomes; `errcheck` stays clean.
- `0o755` for directories, `0o644` for files (`.gitignore` writes already use it, `gitignore.go:66`).
- English only; messages use the existing `—` em dash style of the current hints.

---

## 5. Phased work breakdown

Each phase is a separate PR and a separate release-please entry; each is shippable on its own.

### Phase 0 — prerequisite fixes (patch releases, `fix:` commits)

| # | Item | Fact | Files | Notes |
| --- | --- | --- | --- | --- |
| 0.1 | `Args: cobra.NoArgs` on `gogo git update` | 4 | `internal/cli/git_update.go`, `git_update_test.go` | Behavior change: positional args now error. Commit body states it. |
| 0.2 | `loop.RunEach`; extract `cloneMissing`; `git update` honors `--parallel` / `--concurrency` | 5 | `internal/loop/loop.go`, `loop_test.go`, `internal/cli/clone.go`, `clone_test.go`, `git_update.go`, `git_update_test.go` | `runParallel` refactored onto `RunEach`; existing loop tests must stay green unchanged. |
| 0.3 | `gogo migrate --dry-run` returns the real run's exit code | 3 | `internal/cli/migrate.go` (`:223-226`), `migrate_test.go` | Move the `if dryRun` block below the exit decision; print `Dry run: …` then `return 1` when missing/ambiguous is non-empty. |
| 0.4 | Missing project directory reported by `loop.Loop` | 6 | `internal/loop/loop.go`, `loop_test.go` | Benefits every loop command (`exec`, `run`, `git *`, `npm *`). Exit code unchanged (was already 1). |

0.1–0.4 are independent of each other and can land in any order.

**Implementation note (2026-09-08).** All four are implemented and verified end-to-end against the
`v2` repro; `go test ./...` and `golangci-lint run` are clean. Two things differed from the plan as
written:

- **0.2 —** `cloneMissing` must *not* return a non-nil error from its `RunEach` callback for a
  per-target failure. `RunEach` cancels the shared context on error, which would abort the remaining
  clones; today a bad URL or a failed clone lets the others proceed. Per-target failures are
  therefore returned as `&executor.Result{ExitCode: 1, Stderr: …}`, preserving `git_update.go`'s
  current isolation. Covered by `TestCloneMissingIsolatesPerTargetFailures`.
- **0.4 —** the plan said the existing loop tests stay green unchanged; they do not. Seven tests in
  `internal/loop/loop_test.go` never created the project directories on disk, so the guard made them
  fail (four) or pass for the wrong reason (three, notably `TestLoopWithFailures`, whose asserted
  failure would have come from the missing directory rather than the command). They now create their
  directories via a `mkProjects` helper, which is what the real system always guarantees.

### Phase 1 — `gogo update` (minor release, `feat(cli): add gogo update`)

| # | Item | Files |
| --- | --- | --- |
| 1.1 | `internal/gitstate` package with parser and plumbing + table tests | `internal/gitstate/gitstate.go`, `gitstate_test.go` |
| 1.2 | Extract `planMigration` / `applyMoves` from `runMigrate`; no behavior change | `internal/cli/migrate.go`, `migrate_test.go` |
| 1.3 | `update.go`: command, pipeline, pull policy, summary; register in root | `internal/cli/update.go`, `update_test.go`, `root.go` |
| 1.4 | Hint wording (§7), `hints.go`, `loop.MissingDirectoryMessage` text | `internal/cli/hints.go`, `validate.go`, `migrate.go`, `project_import.go`, `internal/loop/loop.go`, their tests |
| 1.5 | Docs (§8) | `README.md`, `CLAUDE.md`, `SECURITY.md` |

1.1 and 1.2 can be reviewed as separate commits inside the PR; 1.3 depends on both and on Phase 0.2.

**Implementation note (2026-09-08).** Phase 1 is implemented and verified end-to-end against real
git repositories: a working copy with one misplaced project, one missing project and one project
behind its upstream converges in a single `gogo update`, and a second run is a no-op. The pull
policy was confirmed against real repositories too — a diverged project is refused, a dirty working
tree whose changes do not overlap is fast-forwarded with the work in progress intact, and a detached
HEAD is skipped without failing the run.

Two deviations from the design as written:

- `output.ProjectStatus` renders only success and error (`output.go:91-108`), so *skipped* outcomes
  are printed with `output.Warning`, matching how `gogo migrate` already reports missing projects.
- Open question 12 turned out to be load-bearing rather than cosmetic; see its resolution above.

### Phase 2 — follow-ups (each optional, each its own design note)

| # | Item | Depends on |
| --- | --- | --- |
| 2.1 | `--recursive` / `--max-depth` (Decision 1b) for `gogo update` and `gogo git clone` | 1.3 |
| 2.2 | `--strict`: skipped outcomes exit 1 (CI use) | 1.3 |
| 2.3 | Remote-URL normalization for conflict and move matching (ssh vs https, trailing `.git`) | 1.2 |
| 2.4 | Re-plan after moves so a conflict that a move would resolve no longer aborts the move phase | 1.2 |
| 2.5 | `gogo git pull --ff-only` or adoption of the Decision 5 policy by `gogo git pull` | 1.1 |
| 2.6 | `Args: cobra.NoArgs` on `git status`, `git pull`, `git push`, `git commit`, `npm install`, `npm link` | — |

---

## 6. Files to create / modify

**Create**

| Path | Purpose |
| --- | --- |
| `internal/cli/update.go` | `newUpdateCmd`, `updateOptions`, `runUpdate`, `pullProject`, `summarizeUpdate` |
| `internal/cli/update_test.go` | Pipeline, policy, exit-contract and flag tests; `scriptedExecutor` helper |
| `internal/cli/clone.go` | `cloneTarget`, `cloneOutcome`, `cloneMissing` |
| `internal/cli/clone_test.go` | Clone phase tests |
| `internal/cli/hints.go` | Shared hint constants |
| `internal/gitstate/gitstate.go` | `Status`, `Inspect`, `ParsePorcelainV2`, `Fetch`, `AheadBehind`, `FastForward` |
| `internal/gitstate/gitstate_test.go` | Parser and plumbing tests |
| `docs/plans/gogo-update.md` | This document |

**Modify**

| Path | Change |
| --- | --- |
| `internal/cli/root.go:39-48` | `rootCmd.AddCommand(..., newMigrateCmd(), newUpdateCmd(), newGitCmd(), ...)` |
| `internal/cli/git_update.go` | `Args: cobra.NoArgs`; body uses `cloneMissing` with the parallel flags |
| `internal/cli/migrate.go` | Extract `planMigration` / `applyMoves`; dry-run exit parity; use `notFoundHint` |
| `internal/cli/validate.go:96` | Constant moves to `hints.go`; wording |
| `internal/cli/project_import.go:112` | Wording via `cloneLaterHint` |
| `internal/loop/loop.go` | `RunEach`; `runParallel` on top of it; missing-dir guard in `Loop`; `MissingDirectoryMessage` |
| `internal/loop/loop_test.go` | New tests (§7); existing ones unchanged |
| `internal/cli/migrate_test.go` | `TestMigrateDryRunExitMatchesRealRun`; `TestMigrateMissingExitsNonZero` asserts the new wording; `planMigration` table test |
| `internal/cli/git_update_test.go` | NoArgs and parallel tests |
| `internal/cli/validate_test.go:61` | `assert.Contains(..., "gogo update")` instead of `"gogo migrate"` |
| `internal/cli/group_test.go:90` | Add `update` to `TestGroupFlagPositions` |
| `README.md`, `CLAUDE.md`, `SECURITY.md` | §8 |

Nothing under `internal/config`, `internal/filter`, `internal/executor`, `internal/discover`,
`internal/giturl` or `internal/ssh` changes.

---

## 7. Test plan

Style: `t.TempDir()`, table-driven with `testify/assert` + `require`, `captureOutput(t)`
(`migrate_test.go:45`) for output, `initTestChdir(t, dir)` (`init_test.go:14`) for cobra-level tests,
`config.SetOverlayFiles(nil)` where the config is read through the global overlay list, real
directories with a `.git` subdirectory via `mkGitRepo` (`migrate_test.go:38`), and no real git.

**`scriptedExecutor`** (new, in `update_test.go`, package `cli`): implements `executor.Executor`;
keyed by `cwd + "\x00" + strings.Join(append([]string{name}, args...), " ")`; each entry holds a
`*executor.Result` and an optional `func()` side effect (a scripted `git clone` creates
`<parent>/<base>/.git`); records calls under a mutex and tracks `inFlight` / `maxInFlight` to
assert concurrency bounds, as `mockExecutor` does in `loop_test.go:19-41`. Unscripted calls return
`ExitCode: 1, Stderr: "unscripted: <key>"` so a missing expectation fails loudly.

### `internal/gitstate/gitstate_test.go`

| Test | Cases |
| --- | --- |
| `TestParsePorcelainV2` (table) | `branch with upstream, clean`; `detached`; `no upstream`; `initial commit`; `dirty tracked (1 …)`; `renamed (2 …)`; `unmerged (u …)`; `untracked only (? …)`; `empty output → error`; `unknown header ignored` |
| `TestInspectInvokesStatusPorcelain` | argv is exactly `git status --porcelain=v2 --branch`, `Cwd` is the dir |
| `TestAheadBehind` (table) | `"0\t0"`, `"3\t0"`, `"0\t2"`, `"1\t1"`, non-numeric → error, non-zero exit → error |
| `TestFastForwardInvokesMergeFFOnly` | argv is `git merge --ff-only @{u}` |
| `TestFetchUsesNoArguments` | argv is `git fetch` |

### `internal/loop/loop_test.go`

| Test | Asserts |
| --- | --- |
| `TestRunEachSequentialPreservesOrder` | results indexed by input order, `maxInFlight == 1` |
| `TestRunEachParallelIsBounded` | 6 items, concurrency 2 → `maxInFlight == 2`, all 6 results present |
| `TestRunEachErrorCancelsRemaining` | an `error` from `fn` cancels the shared context; later items observe `ctx.Err() != nil`; every item still has a `Result` (mirrors `TestParallelKeepsResultsWhenOneErrors`, `loop_test.go:219`) |
| `TestLoopReportsMissingDirectory` | project dir absent → `Success == false`, `Stderr` contains `directory missing`, command fn never called |
| `TestLoopParallel`, `TestLoopWithFailures`, … | unchanged and green after the `runParallel` refactor |

### `internal/cli/clone_test.go` and `git_update_test.go`

| Test | Asserts |
| --- | --- |
| `TestGitUpdateRejectsPositionalArgs` | `cmd.SetArgs([]string{"libs/web"})` → `cmd.Execute()` errors; no clone call recorded |
| `TestGitUpdateHonorsParallel` | three missing projects, `--parallel --concurrency 2` → `maxInFlight == 2` |
| `TestGitUpdateSequentialByDefault` | same fixture, no flag → `maxInFlight == 1` |
| `TestCloneMissingRejectsInvalidURL` (table) | `""`, `"-oProxyCommand=…"`, `"ext::sh -c id"` → failed outcome, executor not called |
| `TestCloneMissingCreatesParentDirectory` | `libs/web` target → `libs` exists with `0o755` before `git clone` runs in it |
| `TestCloneMissingPropagatesGitStderr` | non-zero exit with stderr → `Err == stderr`; empty stderr → `Err == "clone failed"` |
| `TestGitUpdateExcludesLocalProjectDirs`, `TestGitUpdatePrunesRemovedLocalProject` | existing (`git_update_test.go:13`, `:44`), unchanged |

### `internal/cli/migrate_test.go`

| Test | Asserts |
| --- | --- |
| `TestMigrateDryRunExitMatchesRealRun` (table) | `{moves only → 0/0}`, `{missing only → 1/1}`, `{moves + missing → 1/1}`, `{ambiguous → 1/1}`; dry run leaves the tree untouched in every row |
| `TestPlanMigrationBuckets` (table) | one fixture per bucket: present, move, missing, ambiguous, conflict (different origin), conflict (no origin) |
| `TestMigrateMissingExitsNonZero` | updated to assert `gogo update` in the hint |
| everything else | unchanged |

### `internal/cli/update_test.go`

| Test | Fixture → assertion |
| --- | --- |
| `TestUpdateNotARepo` | plain temp dir → error contains `Not in a gogo-meta repository` |
| `TestUpdateNoProjectsMatchFilter` | `--include-only nope` → warning, code 0, no executor calls |
| `TestUpdateAllPhasesDisabled` | all three `--no-*` → error, code 1 |
| `TestUpdateMovesClonesAndPulls` | the fact-2 scenario: config `libs/api`, `libs/web`; `oldapi` has api's origin; web absent → `libs/api` renamed, `libs/web` cloned, fetch+rev-list run in `libs/api` only, **code 0** |
| `TestUpdateMissingWithNoClone` | absent project + `--no-clone` → outcome skipped, no clone call, code 0 |
| `TestUpdateMisplacedWithNoMigrate` | misplaced project + `--no-migrate` → failed, no rename, no clone call, code 1 |
| `TestUpdateConflictAbortsMovesOnly` | one conflict + one pending move + one missing + one present → no rename, clone happens, pull happens, both conflict and move failed, code 1 |
| `TestUpdateAmbiguousURLIsNotCloned` | URL at two paths → failed, no clone call, code 1 |
| `TestUpdateDryRunChangesNothing` | move + missing + present → no rename, no `git clone`, no `git fetch`, no `git merge`; `Would move`, `Would clone`, `Would pull` printed; code 0 |
| `TestUpdateDryRunExitCode` (table) | `{conflict → 1}`, `{ambiguous → 1}`, `{misplaced + --no-migrate → 1}`, `{missing → 0}`, `{moves → 0}` |
| `TestUpdatePullPolicy` (table) | scripted status/fetch/rev-list/merge per row: `detached → skipped/0`, `no upstream → skipped/0`, `fetch fails → failed/1`, `0/0 → up to date`, `ahead only → up to date (ahead N)`, `behind only → merge called, updated (N)`, `diverged → failed, merge not called`, `merge refused → failed with stderr`, `dirty + refused → message has "(working tree has local changes)"`, `timed out → failed` |
| `TestUpdateFreshClonesAreNotPulled` | cloned project gets no `git status`/`git fetch` call |
| `TestUpdateNoPullSkipsPullPhase` | `--no-pull` → no status/fetch/merge calls; code from other phases |
| `TestUpdateRespectsFilters` (table) | `--include-only`, `--exclude-only`, `--include-pattern`, `--group` (config with `groups`) → only selected projects appear in outcomes and executor calls |
| `TestUpdateParallelPullsAreBounded` | 5 present projects, `--parallel --concurrency 2` → `maxInFlight == 2`; output order equals config order |
| `TestUpdateSyncsLocalExcludes` | `.gogo.local` project, `.git/info` present → managed block contains it, `.gitignore` does not (mirrors `TestGitUpdateExcludesLocalProjectDirs`); also holds under `--include-only` that excludes the local project |
| `TestUpdateMoveUpdatesGitignoreAndExclude` | non-local move: `.gitignore` loses `from`, gains `to`; local move: goes to `.git/info/exclude` only; empty parent pruned (mirrors `TestMigrateUpdatesGitignore`, `TestMigrateLocalProjectMoveGoesToGitExclude`, `TestMigratePrunesEmptyParent`) |
| `TestUpdateSummaryLine` | counts line matches outcomes; `Summary` lists failed projects |
| `TestUpdateCommandRegistered` | `NewRootCommand("test")` has a subcommand `update` with the flags of §3.2 and `Args` rejecting positionals |
| `TestGroupFlagPositions` | extended with `update` (`group_test.go:90`) |

### Integration (existing pattern, real git available)

One `//go:build integration`-style test, or a `Makefile` target, that runs `gogo update` twice on a
temp meta repo with two local bare remotes: first run clones; a commit is pushed to one bare
remote; second run reports `updated (1 commit)` for that project and `up to date` for the other,
exit 0. This is the only test that exercises the real `git` binary; everything above uses the
scripted executor. (Commits in temp repos need `-c commit.gpgsign=false` on machines with signing.)

---

## 8. Message wording changes

| Location | Today | After Phase 1 |
| --- | --- | --- |
| `internal/cli/validate.go:96` (`missingDirectoryHint`) | `directory missing — run 'gogo migrate' if it moved, or 'gogo git update' to clone` | `directory missing — run 'gogo update' to move or clone it` |
| `internal/cli/migrate.go:220` | `%s not found in working copy — run 'gogo git update' to clone` | `%s not found in working copy — run 'gogo update' to clone it` |
| `internal/cli/project_import.go:112` | `Run "gogo git update" to clone missing projects` | `Run "gogo update" to clone missing projects` |
| `internal/loop/loop.go` (new, Phase 0.4) | *(empty failure)* | Phase 0.4: `directory missing — run 'gogo git update' to clone it`; Phase 1: `directory missing — run 'gogo update' to clone it` |
| `internal/cli/migrate.go:224` (Phase 0.3) | `Dry run: %d move(s) pending` then `return 0` | same line, then the real run's exit code |

New messages introduced by `gogo update` are the ones listed in §3.4. `gogo git update` keeps
`Checking N repositories...`, `All repositories are already cloned`, `cloned`, `clone failed`.

`gogo migrate` itself keeps pointing at `gogo update` rather than `gogo git update` because
`gogo update` is the command that cannot bounce the user back.

---

## 9. Documentation updates

**`README.md`**

- Features (`:14-22`): add `Converge the working copy on the config with one command (\`gogo update\`)`.
- Quick Start (`:86-102`): add `gogo update` after the clone step.
- Commands: new `### \`gogo update\`` section between `gogo run` (`:350`) and `gogo git clone`
  (`:388`): description of the three phases, examples (`gogo update`, `gogo update --parallel`,
  `gogo update --dry-run`, `gogo update --no-clone --group frontend`), flag table (§3.2), a short
  exit-code paragraph (§3.4 rules a–c) and the pull policy table (Decision 5).
- `gogo git update` (`:406-424`): state that it takes no positional arguments (use
  `--include-only`); note that `--parallel` now applies (after Phase 0.2); add "For move + clone +
  pull in one step see `gogo update`".
- `gogo git pull` (`:445-462`): add "Runs plain `git pull` with your git configuration; for a
  fast-forward-only update with per-repository reporting see `gogo update`."
- `.gogo.local` paragraph (`:267`): `\`gogo git update\` and \`gogo update\` auto-add local-only …`.
- `gogo project import` example (`:580`): `clone later with gogo update`.
- `gogo validate` (`:682-692`): mention the working-copy check and the new hint.
- `gogo migrate` (`:694-707`): note that `--dry-run` exits like the real run; point to `gogo update`.
- Daily Development Workflow (`:722-741`): replace `gogo git pull --parallel` with
  `gogo update --parallel` and keep `gogo git pull` as the alternative.
- Project Structure (`:778-796`): add `│   ├── gitstate/      # Read-only git state and fast-forward`.

**`CLAUDE.md`**

- Project Structure block: add `update.go`, `clone.go`, `hints.go` under `internal/cli/` and the
  `gitstate/` package with `gitstate.go`, `gitstate_test.go`.
- CLI Usage block: add `gogo update [--dry-run] [--no-migrate|--no-clone|--no-pull] [--parallel]  # Move, clone, fast-forward`.
- New section after "Groups":

  > ### Update pipeline
  > `gogo update` runs `planMigration` (shared with `gogo migrate`), then `applyMoves`, then
  > `cloneMissing` (shared with `gogo git update`), then `pullProject` per present project using
  > `internal/gitstate`. It never merges, rebases or stashes: a project is fast-forwarded or reported.
  > Exit 1 only for failed projects; skipped ones (detached HEAD, no upstream, `--no-clone`) exit 0.
  > Hint strings that name commands live in `internal/cli/hints.go`.

**`SECURITY.md:23`**: add `gogo update` to the list of commands that act on config-supplied URLs.

---

## 10. Open questions

1. **Remote-URL matching is exact string comparison** (`migrate.go:151`, `:160`). A repo cloned via
   `https://github.com/org/api.git` under a config that says `git@github.com:org/api.git` is a
   *conflict* today and will remain one in `gogo update`, which aborts the move phase. Should
   `planMigration` normalize (scheme, host, trailing `.git`, case)? Proposed as Phase 2.3; not
   assumed here.
2. **An existing empty directory at a configured path** is a conflict (`found no remote`). `git clone`
   into an empty directory succeeds. Should an empty directory count as missing?
3. **`--strict`** (skipped → exit 1) for CI workspaces that must be fully converged. Phase 2.2.
4. **Should the clone phase add non-local project paths to `.gitignore`**, as `gogo project import`
   does (`project_import.go:104`)? `gogo git update` does not today; a hand-edited `.gogo` therefore
   leaves the clone untracked-but-visible in the meta repo. Parity would be a small change in
   `cloneMissing`; kept out to avoid changing `gogo git update`'s side effects in the same release.
5. **Streaming progress under `--parallel`**: the design buffers per-phase output like `runParallel`.
   Long clones show nothing until the phase ends. Acceptable for a first release?
6. **Per-command timeout**: `executor.DefaultTimeout` (5 min) applies per clone/fetch/merge. Large
   repositories may need `--timeout`. Not added.
7. **Should `gogo git pull` adopt the fast-forward policy** (or a `--ff-only` flag)? It stays a
   plain `git pull` here so its behavior does not change under users' feet. Phase 2.5.
8. **`Args: cobra.NoArgs` on the other loop commands** that lack `Args:` (`git status`, `git pull`,
   `git push`, `git commit`, `npm install`, `npm link`). Same defect as fact 4; same one-line fix
   each; bundled as Phase 2.6 or folded into 0.1 at the maintainer's discretion.
9. **Recursion flag name and depth default** for Decision 1b (`--recursive`, `--max-depth 8`?), and
   whether root filters may address nested paths.
10. **Submodules**: neither clone nor fast-forward initializes or updates submodules. Out of scope;
    document or add `--recurse-submodules` later?
11. ~~**`gogo migrate` exit code for missing projects**~~ — **RESOLVED 2026-09-08: keep 1.** The
    existing contract stands; `gogo update` owning the missing case does not change what
    `gogo migrate` reports. Phase 0.3 is therefore a `--dry-run` parity fix only.
12. ~~**Output for an unborn branch**~~ — **RESOLVED 2026-09-08 by experiment: fast-forward without
    comparing.** Against real git, an unborn branch that has an upstream (clone of a then-empty
    remote, remote since gained commits) behaves as follows: `git rev-list --left-right --count
    HEAD...@{u}` fails with exit 128 (`fatal: bad revision 'HEAD...'`), while `git merge --ff-only
    @{u}` succeeds and checks the files out. `pullProject` therefore skips `AheadBehind` when
    `Status.Unborn()` and goes straight to the fast-forward; comparing first would leave such a
    repository permanently unconverged. Covered by the `unborn branch fast-forwards without
    comparing` case in `TestUpdatePullPolicy`.

---

## Appendix A — `git status --porcelain=v2 --branch` reference

```
# branch.oid 4f2a…                # or "(initial)"
# branch.head main                # or "(detached)"
# branch.upstream origin/main     # absent when no upstream
# branch.ab +1 -3                 # absent when no upstream; stale until fetch
1 .M N... 100644 100644 100644 <sha> <sha> path      # changed
2 R. N... 100644 100644 100644 <sha> <sha> R100 new\told   # renamed/copied
u UU N... 100644 100644 100644 100644 <sha> <sha> <sha> path  # unmerged
? untracked/file
```

`gitstate.ParsePorcelainV2` reads the three headers it needs and sets `Dirty` on any entry line.

## Appendix B — git plumbing used per present project

| Step | Command | Cwd | Read-only |
| --- | --- | --- | --- |
| Inspect | `git status --porcelain=v2 --branch` | project | yes |
| Fetch | `git fetch` | project | updates remote-tracking refs only |
| Compare | `git rev-list --left-right --count HEAD...@{u}` | project | yes |
| Fast-forward | `git merge --ff-only @{u}` | project | moves the branch and checks out the tree |

`--dry-run` runs only the first step.

## Appendix C — mapping of validated facts to work items

| Fact | Resolved by |
| --- | --- |
| 1 | Phase 1.4 wording; `gogo update` as the target |
| 2 | Phase 1.3 (`gogo update` moves and clones in one run, exit 0); Phase 1.4 wording |
| 3 | Phase 0.3 |
| 4 | Phase 0.1 |
| 5 | Phase 0.2 |
| 6 | Phase 0.4 |
| 7 | Decision 1 (a) now, Phase 2.1 later |
| 8 | Phase 1.4 |
