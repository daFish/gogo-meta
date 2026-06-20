# Readiness Blockers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the two readiness blockers — `gogo init` panic and `.gogo` path traversal — and document the intentional TypeScript divergences in the README.

**Architecture:** Drop init's `-f` shorthand (cobra can't carry it alongside root's persistent `-f`/`--file`). Add a `config.IsSafeProjectPath` guard enforced centrally in `config.Validate` (so every config consumer inherits it) plus on the `folder` CLI arg of `project create`/`import`. Add a README "Differences from the TypeScript version" section.

**Tech Stack:** Go 1.26, cobra, testify, golangci-lint v2.

## Global Constraints

- `gogo init` must not panic; `--force` still works; root `-f`/`--file` unchanged.
- Project paths (config keys + `project create`/`import` `folder` arg) must be relative and stay within the repo; reject absolute or `..`-escaping paths.
- `IsSafeProjectPath` rule: clean the path; reject if `""`, `"."`, `".."`, absolute, or starts with `".." + separator`.
- Intentional divergences from TS (no path validation; TS init accepts `-f`); document in README, soften the "identical CLI behavior" intro line.
- Idiomatic Go; errcheck/staticcheck/gocritic; `make lint` (0 issues) + `make test` green before each commit.
- **Commit style: headline only — NO `Issue:` trailer, NO `Co-Authored-By:` trailer.** Never push.
- Task order: Task 1 (config helper) before Task 2 (which consumes it).

---

## Task 1: `config.IsSafeProjectPath` + enforce in `Validate`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `func IsSafeProjectPath(p string) bool`; `config.Validate` rejects any config whose project keys include an unsafe path.

- [ ] **Step 1: Write failing tests**

Add to `internal/config/config_test.go`:

```go
func TestIsSafeProjectPath(t *testing.T) {
	safe := []string{"libs/api", "a/b/c", "a/../b", "api"}
	for _, p := range safe {
		assert.True(t, IsSafeProjectPath(p), "expected safe: %q", p)
	}
	unsafe := []string{"../x", "../../x", "/abs", "a/../../b", "..", "", ".", "/"}
	for _, p := range unsafe {
		assert.False(t, IsSafeProjectPath(p), "expected unsafe: %q", p)
	}
}

func TestValidateRejectsUnsafeProjectPath(t *testing.T) {
	err := Validate(MetaConfig{Projects: map[string]string{"../evil": "git@x:o/r.git"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project path")

	require.NoError(t, Validate(MetaConfig{Projects: map[string]string{"libs/api": "git@x:o/r.git"}}))
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/config/ -run 'TestIsSafeProjectPath|TestValidateRejectsUnsafeProjectPath' -v`
Expected: FAIL — `IsSafeProjectPath` undefined; `Validate` does not yet reject `../evil`.

- [ ] **Step 3: Add `IsSafeProjectPath`**

In `internal/config/config.go` (near `Validate`; `filepath` and `strings` are already imported):

```go
// IsSafeProjectPath reports whether p is a safe, repository-relative project
// path: non-empty, not "." or "..", not absolute, and not escaping the
// repository via a leading "..".
func IsSafeProjectPath(p string) bool {
	c := filepath.Clean(p)
	if c == "" || c == "." || c == ".." || filepath.IsAbs(c) {
		return false
	}
	return !strings.HasPrefix(c, ".."+string(filepath.Separator))
}
```

- [ ] **Step 4: Enforce in `Validate`**

In `internal/config/config.go`, add the project-path loop to `Validate` (keep the existing `projects is required` check and the command checks unchanged):

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
		if cmd.Cmd == "" {
			return fmt.Errorf("command %q: cmd is required", name)
		}
		if cmd.Concurrency != nil && *cmd.Concurrency <= 0 {
			return fmt.Errorf("command %q: concurrency must be a positive integer", name)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run — expect PASS**

Run: `go test ./internal/config/ -run 'TestIsSafeProjectPath|TestValidateRejectsUnsafeProjectPath' -v`
Expected: PASS.

- [ ] **Step 6: Lint + full test + commit**

Run: `make lint && make test`
Expected: 0 issues; all PASS. (If any existing fixture/test uses an absolute or `..` project key, it was itself unsafe — fix the fixture to a relative path.)

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): reject unsafe (absolute/traversal) project paths"
```

---

## Task 2: Fix `init` panic + guard `project create`/`import` folder

**Files:**
- Modify: `internal/cli/init.go`
- Modify: `internal/cli/project_create.go`
- Modify: `internal/cli/project_import.go`
- Test: `internal/cli/init_test.go`, `internal/cli/project_create_test.go`

**Interfaces:**
- Consumes: `config.IsSafeProjectPath` (Task 1); `NewRootCommand(version string) *cobra.Command`.
- Produces: `gogo init` no longer panics; `project create`/`import` reject unsafe `folder` args.

- [ ] **Step 1: Write failing tests**

Create `internal/cli/init_test.go` (package `cli`). The panic only reproduces through the full root command (root's persistent `-f` merges into `init`), so drive it via `NewRootCommand`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/daFish/gogo-meta/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestChdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// silenceOutput redirects output.Writer/ErrWriter to a buffer for the test.
func silenceOutput(t *testing.T) {
	t.Helper()
	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	t.Cleanup(func() { output.Writer, output.ErrWriter = oldW, oldE })
}

func TestInitDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	initTestChdir(t, dir)
	silenceOutput(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"init"})
	require.NotPanics(t, func() { _ = root.Execute() })
	assert.FileExists(t, filepath.Join(dir, ".gogo"))
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	initTestChdir(t, dir)
	silenceOutput(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"keep":"git@x:o/r.git"}}`), 0o644))

	root := NewRootCommand("test")
	root.SetArgs([]string{"init", "--force"})
	require.NoError(t, root.Execute())

	b, err := os.ReadFile(filepath.Join(dir, ".gogo"))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "keep") // overwritten to default empty config
}
```

Add to `internal/cli/project_create_test.go`:

```go
func TestProjectCreateRejectsUnsafeFolder(t *testing.T) {
	err := runProjectCreate(nil, []string{"../evil", "git@x:o/r.git"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project folder")
}
```

(`runProjectCreate` returns at the folder guard before it touches `cmd`, so passing `nil` is safe.)

- [ ] **Step 2: Run — expect FAIL (init test panics; folder test passes-without-guard?)**

Run: `go test ./internal/cli/ -run 'TestInitDoesNotPanic|TestInitForceOverwrites|TestProjectCreateRejectsUnsafeFolder' -v`
Expected: `TestInitDoesNotPanic` FAILs (panic: `unable to redefine 'f' shorthand`); `TestProjectCreateRejectsUnsafeFolder` FAILs (no guard yet — `runProjectCreate(nil,…)` panics on `cmd.Context()` or proceeds without the error).

- [ ] **Step 3: Drop init's `-f` shorthand**

In `internal/cli/init.go`:

```go
// was:
cmd.Flags().BoolP("force", "f", false, "Overwrite existing config file")
// becomes:
cmd.Flags().Bool("force", false, "Overwrite existing config file")
```

- [ ] **Step 4: Guard the `folder` arg in `project create`**

In `internal/cli/project_create.go`, immediately after `url := args[1]` (before `requireMetaDir`):

```go
	if !config.IsSafeProjectPath(folder) {
		return fmt.Errorf("invalid project folder %q: must be relative and stay within the repository", folder)
	}
```

(`config` and `fmt` are already imported.)

- [ ] **Step 5: Guard the `folder` arg in `project import`**

In `internal/cli/project_import.go`, immediately after the `url` assignment block (before `requireMetaDir`):

```go
	if !config.IsSafeProjectPath(folder) {
		return fmt.Errorf("invalid project folder %q: must be relative and stay within the repository", folder)
	}
```

(Confirm `config` and `fmt` are imported in `project_import.go`; add if missing.)

- [ ] **Step 6: Run — expect PASS**

Run: `go test ./internal/cli/ -run 'TestInitDoesNotPanic|TestInitForceOverwrites|TestProjectCreateRejectsUnsafeFolder' -v`
Expected: PASS.

- [ ] **Step 7: Manual init smoke + lint + full test**

```bash
go build -o /tmp/gogo-rb ./cmd/gogo
d=$(mktemp -d); (cd "$d" && /tmp/gogo-rb init && /tmp/gogo-rb init; /tmp/gogo-rb init --force); rm -rf "$d" /tmp/gogo-rb
```
Expected: first `init` creates `.gogo` (no panic); second prints "already exists"; `--force` overwrites.

Run: `make lint && make test`
Expected: 0 issues; all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/init.go internal/cli/project_create.go internal/cli/project_import.go internal/cli/init_test.go internal/cli/project_create_test.go
git commit -m "fix(cli): unbreak gogo init (drop -f) and reject unsafe project folders"
```

---

## Task 3: README — Differences from the TypeScript version

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: nothing.
- Produces: a documented divergence section; intro no longer claims "identical".

- [ ] **Step 1: Soften the intro line (line 10)**

Replace:

```
Reimplementation of [gogo-meta](https://github.com/daFish/gogo-meta/tree/44344a19bfc70995b142f49a51316dbe126e9f8f) — originally written in TypeScript and now rewritten in Go with identical CLI behavior.
```

with:

```
Reimplementation of [gogo-meta](https://github.com/daFish/gogo-meta/tree/44344a19bfc70995b142f49a51316dbe126e9f8f) — originally written in TypeScript and now rewritten in Go with near-identical CLI behavior (see [Differences from the TypeScript version](#differences-from-the-typescript-version)).
```

- [ ] **Step 2: Add the divergence section after `## Features`**

Insert, immediately before `## Installation`:

```markdown
## Differences from the TypeScript version

The Go rewrite is behavior-compatible with the TypeScript original for all valid inputs.
A few intentional divergences harden security and robustness:

- **Built-in commands run without a shell.** Internal `git` / `npm` / `ssh-keyscan`
  invocations execute as argument vectors, not through `/bin/sh -c`. The TypeScript version
  interpolated values (including project URLs read from `.gogo`) into a shell string, which
  allowed command injection / remote code execution when cloning an untrusted meta
  repository. `gogo exec` and `gogo run` still run shell commands — that is their purpose.
- **Project paths are validated.** Project keys in `.gogo` (and the `folder` argument of
  `gogo project create` / `gogo project import`) must be relative and stay within the
  repository; absolute paths and `..` traversal are rejected. The TypeScript version
  performed no such check, so a malicious `.gogo` could create or move directories outside
  the repository.
- **`gogo init` has no `-f` shorthand.** Use `--force`. (The global `-f` / `--file` overlay
  flag is unchanged.) The TypeScript CLI accepted `-f` for both, which the Go CLI framework
  cannot express without a flag collision.
```

- [ ] **Step 3: Verify + commit**

Confirm the section renders and the anchor matches:
```bash
grep -n 'Differences from the TypeScript version' README.md
grep -n 'near-identical' README.md
```
Expected: both found (the heading + the intro link).

```bash
git add README.md
git commit -m "docs(readme): document intentional divergences from the TypeScript version"
```

---

## Final verification (after all tasks)

- [ ] `make all` green.
- [ ] `gogo init` (built binary) in a temp dir: creates `.gogo`, no panic; `--force` overwrites.
- [ ] Traversal PoC re-run: a `.gogo` with `"../X/y"` key + `gogo git update` fails at config load (`invalid project path`); nothing is created outside the repo.
- [ ] `gogo project create ../evil <url>` → `invalid project folder` error.
- [ ] README has the divergence section; intro says "near-identical".
- [ ] Tag the stage: `git tag -a readiness-blockers -m "fix init panic + path-traversal guard + TS-divergence docs"`.
```
