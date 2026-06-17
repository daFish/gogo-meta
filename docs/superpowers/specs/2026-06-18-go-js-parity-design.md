# Design: Go ↔ JS parity (refactor/go-rewrite → main)

**Date:** 2026-06-18
**Ticket:** gogo-meta-rewrite
**Branch:** refactor/go-rewrite

## Goal

Make the Go implementation on `refactor/go-rewrite` behave **identically** to the
JavaScript/TypeScript implementation at `main` HEAD
(`44344a19bfc70995b142f49a51316dbe126e9f8f`). The JS source is the specification:
port behavior and tests, do not redesign.

## Baseline

The Go rewrite was forked from JS commit `6ae349a`. Commits on `main` since then:

| Commit                          | Type                                                            | Go impact                                 |
|---------------------------------|-----------------------------------------------------------------|-------------------------------------------|
| `487786e`                       | feat(validate): check configured projects exist in working copy | **port**                                  |
| `8e8c05f`                       | feat(migrate): reconcile working copy with configuration        | **port** (new `migrate` cmd + `discover`) |
| `f2522b7`                       | feat(migrate): keep .gitignore in sync when moving repos        | **port** (`RemoveFromGitignore`)          |
| `fc0102d`                       | refactor!: remove legacy `.looprc` support (BREAKING)           | **port** (delete looprc surface)          |
| `e1ac724`                       | fix(deps): commander v15                                        | none (JS toolchain)                       |
| `7218d51`, `e0b777f`, `44344a1` | ci/release tooling                                              | none (JS toolchain)                       |
| `68bc208`, `cbe4f84`, `724e039` | release chores                                                  | none                                      |

Two not-yet-committed specs under `docs/` are **out of scope** — they belong to a
future formal migration to Go.

## Out of scope

- Version-number / release bumps (JS is at 2.0.0; Go versioning is independent).
- CI/release-tooling migration (release-please, Docker CI).
- The two uncommitted `docs/` specs.

---

## Phase 1 — Remove `.looprc` (BREAKING, mirrors `fc0102d`)

`.looprc` is no longer read by JS. Remove the entire surface from Go so
`exec`/`run`/`git`/`npm` no longer apply a `.looprc` ignore list, and `validate`
no longer validates `.looprc`.

**Changes:**

- `internal/config/config.go`: delete `LoopRcFile` const, `LoopRc` type,
  `ValidateLoopRc`, `ReadLoopRc`, `ParseLoopRcContent`.
- `internal/filter/filter.go`: delete `FilterFromLoopRc`.
- `internal/loop/loop.go`: remove the looprc read + filter block (the
  `config.ReadLoopRc` / `filter.FilterFromLoopRc` call, ~lines 47–50). The core
  loop orchestration (sequential/parallel) is otherwise unchanged.
- `internal/cli/validate.go`: delete `validateLoopRcFile`; in `findConfigFiles`
  remove the `hasLoopRc` detection/append so only `.gogo` and `.gogo.*` files are
  collected.

**Behavioral result:** a repo that relied on `.looprc` now runs commands against
the previously-excluded directories. Replacement is `--exclude-only` /
`--exclude-pattern` or editing the `projects` map. Identical to JS.

## Phase 2 — `validate` checks the working copy (mirrors `487786e`)

After config-file validation, `validate` also checks that every configured project
directory exists in the working copy.

**Changes to `internal/cli/validate.go`:**

- Add `validateWorkingCopy(cwd) (bool, error)`-style logic mirroring JS
  `validateWorkingCopy`:
    - Resolve config via `config.ReadMetaConfig` / `config.GetMetaDir`. If that
      fails (not in a meta repo) → return "no errors", skip silently.
    - If `projects` is empty → skip silently.
    - For each project path, if `filepath.Join(metaDir, path)` does not exist →
      `output.ProjectStatus(path, "error", MISSING_DIRECTORY_HINT)` and mark errors.
    - If none missing → `output.Success("All N project directories present")`.
- Constant:
  `MISSING_DIRECTORY_HINT = "directory missing — run 'gogo migrate' if it moved, or 'gogo git update' to clone"`
- Overall failure if **config files had errors OR working copy had errors** →
  return the `validation failed` error (exit code 1). Config-file statuses are
  printed first, then the working-copy section, matching JS ordering.
- Update the command `Short`/`Long` text to mention the working-copy check
  ("Validate config files and check that configured projects exist in the working
  copy").

Note: `fileExists` in `internal/config/config.go` is currently unexported. Either
export it (`FileExists`) for reuse, or use a local `os.Stat` check in validate. The
plan will export it if `migrate` also needs it (it does), keeping one helper.

## Phase 3 — `discover` package + `migrate` command

### 3a. `internal/discover/discover.go` (mirrors `src/core/discover.ts`)

```
func FindGitRepos(rootDir string, ignore []string) ([]string, error)
```

- Recursively walk `rootDir`.
- A directory containing a `.git` entry is a git repo: record its path
  **relative to `rootDir`, POSIX-style (`/` separators)**, and do **not** descend
  into it.
- Never report `rootDir` itself.
- Skip entries named `.git` and any directory whose base name is in `ignore`.
- Unreadable directories are skipped silently (no error propagation), matching the
  JS `try/catch { return }`.
- Return the slice **sorted** ascending.

### 3b. `RemoveFromGitignore` (mirrors `removeFromGitignore` in `config.ts`)

Add to `internal/config/gitignore.go`:

```
func RemoveFromGitignore(metaDir, entry string) (bool, error)
```

- If `.gitignore` absent → return `false, nil`.
- Read file, split on `\n`, drop lines whose trimmed value `== entry`.
- If nothing removed → return `false, nil`.
- Otherwise rewrite (joined with `\n`) and return `true, nil`.

### 3c. `internal/cli/migrate.go` (mirrors `src/commands/migrate.ts`)

Command: `gogo migrate [--dry-run]`
Description: "Move/rename working-copy directories to match the configuration".
`--dry-run`: "Show what would be moved without changing anything".

Algorithm (identical to JS):

1. Resolve `metaDir` via `config.GetMetaDir`. If none →
   error `Not in a gogo-meta repository. Run "gogo init" first.`
2. Read merged config. If `projects` empty →
   `output.Success("Working copy already matches configuration")`, return.
3. Build `urlToPath` map and `ambiguousUrls` set:
    - `repoPaths := discover.FindGitRepos(metaDir, config.Ignore)`.
    - For each, run `git remote get-url origin` (via `executor.Executor`, `cwd =
     join(metaDir, repoPath)`). Skip if exit != 0 or empty stdout (trimmed).
    - First URL → map; duplicate URL → add to `ambiguousUrls`, skip.
4. Classify each desired `(projectPath, url)`:
    - target dir exists:
        - read its `origin` remote; if `!= url` → **conflict**
          `{path, found: remote-or-nil}`.
        - else: already in place, skip.
    - else if `url` in `ambiguousUrls` → **ambiguous** (`projectPath`).
    - else if `urlToPath[url]` exists and `!= projectPath` →
      **move** `{from: currentPath, to: projectPath}`.
    - else → **missing** (`projectPath`).
5. If any conflicts: for each, `output.ProjectStatus(path, "error", "occupied by a
   different repository (found <found-or-'no remote'>)")`, then return error
   `Migration aborted: one or more target paths are occupied by a different
   repository`.
6. If no moves AND no missing AND no ambiguous →
   `output.Success("Working copy already matches configuration")`, return.
7. For each move:
    - dry-run → `output.Info("Would move <from> → <to>")`, continue.
    - else: `os.MkdirAll(dirname(targetDir), 0o755)`, `os.Rename(from, to)`,
      `pruneEmptyParents(metaDir, from)`, `RemoveFromGitignore(metaDir, from)`,
      `AddToGitignore(metaDir, to)`,
      `output.ProjectStatus(to, "success", "moved from <from>")`.
8. For each ambiguous → `output.Warning("<path>: multiple working-copy directories
   share its repository URL — resolve manually")`.
9. For each missing → `output.Warning("<path> not found in working copy — run
   'gogo git update' to clone")`.
10. dry-run → `output.Info("Dry run: N move(s) pending")`, return.
11. If any missing OR ambiguous → exit code 1 (see parity note).

**`pruneEmptyParents(metaDir, movedFrom)`** (mirrors JS): starting from the parent
of `join(metaDir, movedFrom)`, while the dir is strictly inside `metaDir` and
empty, `os.Remove` it and move up one level. Stop at `metaDir` or first non-empty /
unreadable directory.

**Register** the command in `internal/cli/root.go` `AddCommand(...)`.

### Parity note — exit code on missing/ambiguous

JS sets `process.exitCode = 1` **without throwing**: warnings are already printed
and the command otherwise completes. In Go/cobra, returning an error from `RunE`
both prints the error to stderr and sets exit 1 — which would add an extra stderr
line JS does not emit. To keep stdout + exit code identical:

- Print all warnings first (steps 8–9).
- Then signal exit 1 **without** an extra cobra error message — set
  `cmd.SilenceErrors`/`SilenceUsage` and return a sentinel error, OR call the
  process-exit path the other commands already use. The plan will choose whichever
  mechanism the existing Go CLI already uses for "completed with non-zero" so no
  spurious `Error:` line is printed. Net observable behavior (printed output + exit
  code) must equal JS.

---

## Testing — port vitest suites to Go (table-driven, `t.TempDir`)

Mirror the JS test intent; use mocked `executor.Executor` where git is invoked and
override `output.Writer`/`output.ErrWriter` to capture output.

- **`internal/discover/discover_test.go`** — repo detection, no-descend into repos,
  `ignore` by base name, root never reported, POSIX-relative output, sorted.
- **`internal/cli/migrate_test.go`** — the 9 JS cases:
    1. not in a gogo-meta repo → error
    2. already in sync (target exists, matching remote) → success message, no move
    3. move repo from current path to configured path
    4. prune parent dir left empty by a move
    5. keep parent dir that still has other repos
    6. `.gitignore` updated on move (old entry gone, new entry present)
    7. dry-run moves nothing, emits "Would move" / "Dry run"
    8. conflict (target occupied by different repo) → abort error, "occupied"
    9. missing configured repo → warning mentioning `gogo git update`, exit code 1
- **`internal/cli/validate_test.go`** (new or extended) — working copy all present,
  some missing (error + hint), skip when not a meta repo / no projects, combined
  config-error + missing-dir failure.
- **`internal/filter/filter_test.go`** — remove `FilterFromLoopRc` cases.
- **`internal/config/config_test.go`** — remove any `.looprc`/`ReadLoopRc`/
  `ParseLoopRcContent`/`ValidateLoopRc` test references.

All must pass under `make lint` and `make test`.

## Docs

- **`README.md` line 10** — update the reimplementation tree-URL commit hash from
  `6ae349afce42af1081c6c40d64a0affb708ff562` to
  `44344a19bfc70995b142f49a51316dbe126e9f8f`.
- **`CLAUDE.md` line 3** — same hash appears twice (tree URL + "rewritten from
  commit"); update both for consistency. Remove the `.looprc` description from the
  Configuration Files section; add `migrate` to the command list and CLI usage.
- **`README.md`** — remove any `.looprc` documentation; document `gogo migrate`.

## Verification

`make lint && make test` green; new `migrate`/`discover`/`validate` behavior
matches JS case-for-case; no `.looprc` references remain in `internal/`, `cmd/`,
`README.md`, `CLAUDE.md`.
