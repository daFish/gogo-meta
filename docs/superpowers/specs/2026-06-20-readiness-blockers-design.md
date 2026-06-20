# Readiness Blockers: `gogo init` Panic + Path Traversal

**Date:** 2026-06-20
**Branch:** `fix/shell-safety` (continues the security work)

## Goal

Clear the two blockers that make the Go rewrite not-ready-for-review, and document the
intentional divergences from the TypeScript version in the README:

1. **CRITICAL — `gogo init` panics.** `init` declares `-f`/`--force`; root declares a
   persistent `-f`/`--file`. Cobra merges persistent flags into the subcommand on execute
   and pflag rejects the duplicate shorthand: `panic: unable to redefine 'f' shorthand`.
   Every user hits it on first run.
2. **HIGH — path traversal via `.gogo` project keys.** Project paths are interpolated into
   the filesystem with no validation. A `../` (or absolute) key escapes the meta repo:
   `git clone`/`git update`/`migrate` create/move directories outside it. Proven: a config
   `{"projects":{"../tt_ESCAPED/evil":…}}` + `gogo git update` created `/tmp/tt_ESCAPED`.

Plus: a README section making the intentional deviations from the TS version explicit.

## Design

### Blocker 1 — drop init's `-f` shorthand (option 1)

`internal/cli/init.go`:

```go
// was:
cmd.Flags().BoolP("force", "f", false, "Overwrite existing config file")
// becomes:
cmd.Flags().Bool("force", false, "Overwrite existing config file")
```

`gogo init` no longer panics. `--force` still works (overwrite existing config). Root keeps
`-f`/`--file` (the overlay shorthand). On `init`, `-f` now resolves to the inherited
persistent `--file` (harmless — init ignores overlays).

### Blocker 2 — central path-traversal guard

**Helper** in `internal/config/config.go` (metaDir-free; clean-then-check):

```go
// IsSafeProjectPath reports whether p is a safe, repo-relative project path:
// not empty/".", not absolute, and does not escape the repository via "..".
func IsSafeProjectPath(p string) bool {
	c := filepath.Clean(p)
	if c == "" || c == "." || c == ".." || filepath.IsAbs(c) {
		return false
	}
	return !strings.HasPrefix(c, ".."+string(filepath.Separator))
}
```

Rejects: `../x`, `../../x`, `/abs`, `a/../../b` (cleans to `../b`), `..`, ``, `.`.
Allows: `libs/api`, `a/b/c`, `a/../b` (cleans to `b`).

**Enforce in `config.Validate`** — it runs on every parsed config (primary in
`ReadMetaConfig`, `-f` overlays via `ReadOverlayConfig`, and `gogo validate`'s per-file
pass), so one check protects every consumer:

```go
func Validate(config MetaConfig) error {
	if config.Projects == nil {
		return errors.New("projects is required")
	}
	for path := range config.Projects {
		if !IsSafeProjectPath(path) {
			return fmt.Errorf("invalid project path %q: must be relative and stay within the repository", path)
		}
	}
	for name, cmd := range config.Commands {
		// ... existing command checks unchanged ...
	}
	return nil
}
```

A hostile/malformed `.gogo` now fails to load with a clear error; `git clone`/`update`/
`migrate`/`npm link`/`validate` all inherit the protection because they go through
`ReadMetaConfig`/`Validate`.

**Guard the CLI-arg `folder`** in `internal/cli/project_create.go` and
`internal/cli/project_import.go` (these take `folder` from args, not from the config, so
`Validate` does not see it). At the top of each command's run function, after obtaining
`folder`:

```go
if !config.IsSafeProjectPath(folder) {
	return fmt.Errorf("invalid project folder %q: must be relative and stay within the repository", folder)
}
```

### README — `## Differences from the TypeScript version`

Add a new top-level section (placed right after `## Features`) listing the intentional
behavioral divergences, and soften the intro line.

- **Intro line (line 10):** change "now rewritten in Go with identical CLI behavior" →
  "now rewritten in Go with near-identical CLI behavior (see *Differences from the
  TypeScript version*)".
- **New section content** (the three current intentional divergences — all
  security/robustness, behavior-identical for valid inputs):

  > ## Differences from the TypeScript version
  >
  > The Go rewrite is behavior-compatible with the TypeScript original for all valid
  > inputs. A few intentional divergences harden security and robustness:
  >
  > - **Built-in commands run without a shell.** Internal `git`/`npm`/`ssh-keyscan`
  >   invocations execute as argument vectors, not through `/bin/sh -c`. The TypeScript
  >   version interpolated values (including project URLs from `.gogo`) into a shell
  >   string, allowing command injection / RCE when cloning an untrusted meta repository.
  >   `gogo exec` and `gogo run` still run shell commands — that is their purpose.
  > - **Project paths are validated.** Project keys in `.gogo` (and the `folder` argument
  >   of `gogo project create`/`import`) must be relative and stay within the repository.
  >   Absolute paths or `..` traversal are rejected. The TypeScript version performed no
  >   such check, allowing a malicious `.gogo` to create or move directories outside the
  >   repository.
  > - **`gogo init` has no `-f` shorthand.** Use `--force`. (The root `-f`/`--file`
  >   overlay flag is unchanged.) The TypeScript CLI accepted `-f` for both, which the
  >   Go CLI framework cannot express without a flag collision.

## Files Changed

| File | Change |
|------|--------|
| `internal/cli/init.go` | `BoolP("force","f",…)` → `Bool("force",…)`. |
| `internal/config/config.go` | Add `IsSafeProjectPath`; enforce it for project keys in `Validate`. |
| `internal/config/config_test.go` | `IsSafeProjectPath` table; `Validate` rejects a config with an unsafe project key. |
| `internal/cli/project_create.go` | Guard `folder` with `IsSafeProjectPath`. |
| `internal/cli/project_import.go` | Guard `folder` with `IsSafeProjectPath`. |
| `internal/cli/init_test.go` | `gogo init` builds+runs without panic; `--force` overwrites. |
| `internal/cli/project_create_test.go` (or import) | `folder` traversal → error. |
| `README.md` | Add `## Differences from the TypeScript version`; soften the intro line. |

## Parity

All three divergences are intentional and recorded in the parity memory. They are
behavior-identical to TS for valid inputs; they only reject inputs that are unsafe
(traversal/absolute paths, shell metacharacters) or that the Go CLI framework cannot
represent (`init -f`).

## Testing

- `IsSafeProjectPath` table: safe (`libs/api`, `a/b/c`, `a/../b`) and unsafe (`../x`,
  `../../x`, `/abs`, `a/../../b`, `..`, ``, `.`).
- `Validate` returns an error for `{"projects":{"../evil":"…"}}` and passes for a normal
  config.
- `gogo init`: the command tree builds and `runInit` executes without panic; `--force`
  removes an existing config and recreates it.
- `gogo project create ../evil <url>` (and `project import ../evil`) returns the
  invalid-folder error and creates nothing.
- Regression (manual or integration): the earlier PoC — `{"projects":{"../X/y":…}}` +
  `gogo git update` — now fails at config load; no directory is created outside the repo.
- `make lint` (0 issues) and `make test` green per task.

## Verification

1. `make all` green.
2. `gogo init` in a temp dir → creates `.gogo`, no panic; `gogo init` again → "already
   exists"; `gogo init --force` → overwrites.
3. Re-run the traversal PoC → blocked at load (`invalid project path`), nothing created
   outside the repo.
4. README renders a `Differences from the TypeScript version` section; the intro no longer
   claims "identical".
