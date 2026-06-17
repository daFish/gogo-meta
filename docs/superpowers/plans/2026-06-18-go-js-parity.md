# Go ↔ JS Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:
> executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Go implementation on `refactor/go-rewrite` to behavioral parity with the JS implementation at `main`
HEAD (`44344a19bfc70995b142f49a51316dbe126e9f8f`).

**Architecture:** Port four behavioral deltas from JS `main`: remove `.looprc`, add a working-copy presence check to
`validate`, add a `discover` package, and add a `migrate` command with `.gitignore` sync. The JS source is the spec —
mirror behavior and tests; do not redesign.

**Tech Stack:** Go 1.24+, cobra, gopkg.in/yaml.v3, fatih/color, testify, golangci-lint v2.

## Global Constraints

- Behavior must be **identical** to JS `main`; the JS source files are the reference.
- Idiomatic Go; `internal/` packages only; `0o755`/`0o644` octal style.
- Errors handled explicitly (errcheck); `staticcheck` enabled — error string **literals** must not be capitalized (
  ST1005). Where a user-facing message must stay capitalized for parity, store it in a `const` and pass the const to
  `errors.New(...)` (ST1005 only inspects literals, not const identifiers).
- Tests: table-driven, `testify`, `t.TempDir()` isolation; mock `executor.Executor`; override `output.Writer`/
  `output.ErrWriter` to capture output.
- Verify each task with `make lint` and `make test` (both must be green) before committing.
- Conventional Commits; include trailer `Issue: gogo-meta-rewrite`; co-author trailer
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`. Never push.

---

## Task 1: Remove `.looprc` support (BREAKING, mirrors `fc0102d`)

Removing `.looprc` touches four files plus their tests; partial removal breaks compilation, so this is one task with one
test cycle.

**Files:**

- Modify: `internal/config/config.go` (delete `LoopRcFile`, `LoopRc`, `ValidateLoopRc`, `ReadLoopRc`,
  `ParseLoopRcContent`)
- Modify: `internal/filter/filter.go` (delete `FilterFromLoopRc`)
- Modify: `internal/loop/loop.go` (delete looprc read+filter block, ~lines 47–50)
- Modify: `internal/cli/validate.go` (delete `validateLoopRcFile`; drop `hasLoopRc` branch in `findConfigFiles`)
- Test: `internal/filter/filter_test.go`, `internal/config/config_test.go`, `internal/cli/validate_test.go` (if
  present) — remove looprc cases

**Interfaces:**

- Consumes: nothing new.
- Produces: `findConfigFiles(cwd string) ([]string, error)` now returns only `.gogo` / `.gogo.*` entries (sorted). No
  looprc symbols remain in any package.

- [ ] **Step 1: Remove looprc test cases first (so the suite reflects the target)**

Delete from `internal/filter/filter_test.go` every test function/table entry exercising `FilterFromLoopRc`. Delete from
`internal/config/config_test.go` every test referencing `ReadLoopRc`, `ParseLoopRcContent`, `ValidateLoopRc`, `LoopRc`,
or `LoopRcFile`. If `internal/cli/validate_test.go` has `.looprc` cases, delete them too.

Find them:

```bash
grep -rn 'LoopRc\|looprc\|FilterFromLoopRc' internal/
```

- [ ] **Step 2: Run tests — expect COMPILE FAILURE**

Run: `make test`
Expected: build fails — production code still references removed test symbols (or vice versa). This confirms the symbols
are still wired in.

- [ ] **Step 3: Remove `FilterFromLoopRc` from `internal/filter/filter.go`**

Delete the entire `FilterFromLoopRc` function. If `path/filepath` becomes unused after removal, drop the import (it is
still used by `Apply` via `filepath.Base`, so it stays).

- [ ] **Step 4: Remove the looprc block from `internal/loop/loop.go`**

Delete these lines (the `.looprc` integration):

```go
	// Apply looprc filtering.
	loopRc, err := config.ReadLoopRc(loopCtx.MetaDir)
	if err == nil && loopRc != nil && len(loopRc.Ignore) > 0 {
		directories = filter.FilterFromLoopRc(directories, loopRc.Ignore)
	}
```

If `err` was only declared here, ensure no leftover unused variable. Leave the rest of the loop orchestration untouched.

- [ ] **Step 5: Remove looprc symbols from `internal/config/config.go`**

Delete: the `LoopRcFile = ".looprc"` const, the `LoopRc` struct type, `ValidateLoopRc`, `ReadLoopRc`, and
`ParseLoopRcContent`. Remove any import left unused (e.g. if `FindFileUp` is otherwise still used, keep it).

- [ ] **Step 6: Simplify `internal/cli/validate.go`**

Delete the `validateLoopRcFile` function. Replace `findConfigFiles` with the looprc-free version:

```go
func findConfigFiles(cwd string) ([]string, error) {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return nil, err
	}

	var configFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if name == ".gogo" || strings.HasPrefix(name, ".gogo.") {
			configFiles = append(configFiles, name)
		}
	}

	sort.Strings(configFiles)
	return configFiles, nil
}
```

In `runValidate`, remove the `if filename == config.LoopRcFile { ... }` branch so every file goes through
`validateConfigFile`:

```go
	var results []validationResult
	for _, filename := range configFiles {
		filePath := filepath.Join(cwd, filename)
		results = append(results, validateConfigFile(filePath, filename))
	}
```

- [ ] **Step 7: Run lint + tests — expect PASS**

Run: `make lint && make test`
Expected: PASS. No `.looprc` references remain.

```bash
grep -rn 'LoopRc\|looprc' internal/ cmd/ && echo "STILL PRESENT" || echo "clean"
```

Expected: `clean`.

- [ ] **Step 8: Commit**

```bash
git add internal/config internal/filter internal/loop internal/cli/validate.go internal/cli/validate_test.go
git commit -m "$(cat <<'EOF'
refactor(go)!: remove legacy .looprc support

Mirror JS fc0102d: drop LoopRc/ReadLoopRc/FilterFromLoopRc, the per-loop
ignore merge in loop.go, and .looprc handling in validate.

Issue: gogo-meta-rewrite

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `validate` checks the working copy (mirrors `487786e`)

**Files:**

- Modify: `internal/config/config.go` (rename unexported `fileExists` → exported `FileExists`; update internal callers)
- Modify: `internal/cli/validate.go` (add working-copy check, update command text)
- Test: `internal/cli/validate_test.go`

**Interfaces:**

- Consumes: `config.GetMetaDir(cwd) (string, error)`,
  `config.ReadMetaConfig(cwd, nil) (*config.MetaConfigResult, error)` (`.Config.Projects`, `.MetaDir`).
- Produces: `config.FileExists(path string) bool`; `validate` returns error `validation failed` when config files OR
  working-copy dirs have errors.

- [ ] **Step 1: Export `FileExists` in config package**

In `internal/config/config.go` rename `func fileExists(path string) bool` to `func FileExists(path string) bool`. Update
all in-package callers (in `config.go` and `gitignore.go`) from `fileExists(` to `FileExists(`:

```bash
grep -rn 'fileExists(' internal/config/
```

Replace each occurrence. Run `make test` to confirm the package still builds (no behavior change).

- [ ] **Step 2: Write the failing test for the working-copy check**

Add to `internal/cli/validate_test.go` (create the file if it does not exist; package `cli`):

```go
func TestValidateWorkingCopyMissingDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"libs/api":"git@example.com:org/api.git"}}`), 0o644))

	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = oldW, oldE }()

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(wd) }()

	err := runValidate(nil, nil)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "directory missing")
	assert.Contains(t, buf.String(), "gogo migrate")
}

func TestValidateWorkingCopyAllPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"libs/api":"git@example.com:org/api.git"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "libs/api"), 0o755))

	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = oldW, oldE }()

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(wd) }()

	err := runValidate(nil, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All 1 project directories present")
}
```

Ensure imports: `bytes`, `os`, `path/filepath`, `testing`, `github.com/daFish/gogo-meta/internal/output`, testify
`assert`/`require`.

- [ ] **Step 3: Run tests — expect FAIL**

Run: `go test ./internal/cli/ -run TestValidateWorkingCopy -v`
Expected: FAIL — `runValidate` does not yet check the working copy (no "directory missing"; all-present test sees no
success line / wrong result).

- [ ] **Step 4: Implement the working-copy check in `internal/cli/validate.go`**

Add the constant and function:

```go
const missingDirectoryHint = "directory missing — run 'gogo migrate' if it moved, or 'gogo git update' to clone"

// validateWorkingCopy reports whether any configured project directory is
// missing from the working copy. It prints per-project errors and returns true
// when at least one directory is missing. If the cwd is not inside a meta repo
// (or there are no projects), it returns false without output.
func validateWorkingCopy(cwd string) bool {
	result, err := config.ReadMetaConfig(cwd, nil)
	if err != nil {
		return false
	}

	projectPaths := make([]string, 0, len(result.Config.Projects))
	for p := range result.Config.Projects {
		projectPaths = append(projectPaths, p)
	}
	if len(projectPaths) == 0 {
		return false
	}
	sort.Strings(projectPaths)

	hasErrors := false
	for _, projectPath := range projectPaths {
		projectDir := filepath.Join(result.MetaDir, projectPath)
		if !config.FileExists(projectDir) {
			output.ProjectStatus(projectPath, "error", missingDirectoryHint)
			hasErrors = true
		}
	}

	if !hasErrors {
		output.Success(fmt.Sprintf("All %d project directories present", len(projectPaths)))
	}

	return hasErrors
}
```

Wire it into `runValidate` after the config-file status loop, replacing the final error return:

```go
	configHasErrors := hasErrors
	workingCopyHasErrors := validateWorkingCopy(cwd)

	if configHasErrors || workingCopyHasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
```

(Rename the existing `hasErrors` accumulator usage so the config-file result is captured in `configHasErrors`; print all
config-file statuses before calling `validateWorkingCopy` so ordering matches JS.)

Update the command definition text:

```go
func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate config files and check that configured projects exist in the working copy",
		RunE:  runValidate,
	}
}
```

- [ ] **Step 5: Run lint + tests — expect PASS**

Run: `make lint && make test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/gitignore.go internal/cli/validate.go internal/cli/validate_test.go
git commit -m "$(cat <<'EOF'
feat(go,validate): check configured projects exist in working copy

Mirror JS 487786e. Export config.FileExists for reuse.

Issue: gogo-meta-rewrite

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: `discover` package (mirrors `src/core/discover.ts`)

**Files:**

- Create: `internal/discover/discover.go`
- Test: `internal/discover/discover_test.go`

**Interfaces:**

- Consumes: stdlib only.
- Produces: `discover.FindGitRepos(rootDir string, ignore []string) ([]string, error)` — sorted, POSIX-relative paths of
  every git repo under `rootDir` (root excluded, repos not descended into, `ignore` base-name dirs skipped, unreadable
  dirs skipped silently).

- [ ] **Step 1: Write the failing test**

Create `internal/discover/discover_test.go`:

```go
package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkRepo(t *testing.T, root, rel string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, rel, ".git"), 0o755))
}

func TestFindGitRepos(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root, "libs/api")
	mkRepo(t, root, "libs/web")
	mkRepo(t, root, "tools")
	// nested repo inside a repo must NOT be descended into:
	mkRepo(t, root, "libs/api/vendor/inner")
	// ignored dir:
	mkRepo(t, root, "node_modules/pkg")
	// plain dir, no .git:
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))

	got, err := FindGitRepos(root, []string{"node_modules"})
	require.NoError(t, err)
	assert.Equal(t, []string{"libs/api", "libs/web", "tools"}, got)
}

func TestFindGitReposEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := FindGitRepos(root, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./internal/discover/ -v`
Expected: FAIL — `FindGitRepos` undefined / package missing.

- [ ] **Step 3: Implement `internal/discover/discover.go`**

```go
// Package discover walks a directory tree to locate git repositories.
package discover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const gitDir = ".git"

func toPosixRelative(rootDir, dir string) string {
	rel, err := filepath.Rel(rootDir, dir)
	if err != nil {
		rel = dir
	}
	return filepath.ToSlash(rel)
}

// FindGitRepos walks rootDir and returns the paths (relative to rootDir,
// POSIX-style) of every directory that is a git repository. The root itself is
// never reported, discovered repositories are not descended into, and any
// directory whose base name appears in ignore is skipped.
func FindGitRepos(rootDir string, ignore []string) ([]string, error) {
	ignoreSet := make(map[string]bool, len(ignore))
	for _, name := range ignore {
		ignoreSet[name] = true
	}

	var repos []string

	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		isRepo := false
		for _, e := range entries {
			if e.Name() == gitDir {
				isRepo = true
				break
			}
		}
		if isRepo && dir != rootDir {
			repos = append(repos, toPosixRelative(rootDir, dir))
			return
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if e.Name() == gitDir || ignoreSet[e.Name()] {
				continue
			}
			walk(filepath.Join(dir, e.Name()))
		}
	}

	walk(rootDir)
	sort.Strings(repos)
	return repos, nil
}

var _ = strings.TrimSpace // remove if strings ends up unused
```

Remove the trailing `var _ =` line and the `strings` import if `strings` is not used (it is not in this implementation —
delete the import).

- [ ] **Step 4: Run test — expect PASS**

Run: `go test ./internal/discover/ -v`
Expected: PASS.

- [ ] **Step 5: Lint + commit**

Run: `make lint`

```bash
git add internal/discover/
git commit -m "$(cat <<'EOF'
feat(go,discover): add git-repo discovery walker

Mirror JS src/core/discover.ts findGitRepos.

Issue: gogo-meta-rewrite

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `RemoveFromGitignore` helper (mirrors `removeFromGitignore`, `f2522b7`)

**Files:**

- Modify: `internal/config/gitignore.go`
- Test: `internal/config/gitignore_test.go`

**Interfaces:**

- Consumes: `config.FileExists` (from Task 2).
- Produces: `config.RemoveFromGitignore(metaDir, entry string) (bool, error)` — returns true if a line matching
  `entry` (trimmed) was removed.

- [ ] **Step 1: Write the failing test**

Add to `internal/config/gitignore_test.go`:

```go
func TestRemoveFromGitignore(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gi, []byte("node_modules\nlibs/api\ndist\n"), 0o644))

	removed, err := RemoveFromGitignore(dir, "libs/api")
	require.NoError(t, err)
	assert.True(t, removed)

	content, err := os.ReadFile(gi)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "libs/api")
	assert.Contains(t, string(content), "node_modules")
}

func TestRemoveFromGitignoreNoMatch(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gi, []byte("node_modules\n"), 0o644))

	removed, err := RemoveFromGitignore(dir, "libs/api")
	require.NoError(t, err)
	assert.False(t, removed)
}

func TestRemoveFromGitignoreNoFile(t *testing.T) {
	dir := t.TempDir()
	removed, err := RemoveFromGitignore(dir, "libs/api")
	require.NoError(t, err)
	assert.False(t, removed)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./internal/config/ -run TestRemoveFromGitignore -v`
Expected: FAIL — `RemoveFromGitignore` undefined.

- [ ] **Step 3: Implement in `internal/config/gitignore.go`**

```go
// RemoveFromGitignore removes the line matching entry (after trimming) from the
// .gitignore in metaDir. Returns true if a line was removed, false if the file
// is absent or contained no matching line.
func RemoveFromGitignore(metaDir, entry string) (bool, error) {
	gitignorePath := filepath.Join(metaDir, ".gitignore")

	if !FileExists(gitignorePath) {
		return false, nil
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != entry {
			filtered = append(filtered, line)
		}
	}

	if len(filtered) == len(lines) {
		return false, nil
	}

	return true, os.WriteFile(gitignorePath, []byte(strings.Join(filtered, "\n")), 0o644)
}
```

- [ ] **Step 4: Run test — expect PASS**

Run: `go test ./internal/config/ -run TestRemoveFromGitignore -v`
Expected: PASS.

- [ ] **Step 5: Lint + commit**

Run: `make lint`

```bash
git add internal/config/gitignore.go internal/config/gitignore_test.go
git commit -m "$(cat <<'EOF'
feat(go,config): add RemoveFromGitignore helper

Mirror JS removeFromGitignore (f2522b7) for migrate gitignore sync.

Issue: gogo-meta-rewrite

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: `migrate` command (mirrors `src/commands/migrate.ts`, `8e8c05f`)

**Files:**

- Create: `internal/cli/migrate.go`
- Test: `internal/cli/migrate_test.go`

**Interfaces:**

- Consumes: `discover.FindGitRepos`, `config.RemoveFromGitignore`, `config.AddToGitignore`, `config.FileExists`,
  `config.GetMetaDir`, `config.ReadMetaConfig`, `executor.Executor`/`executor.NewShellExecutor`, `output.*`.
- Produces: `newMigrateCmd() *cobra.Command`; testable core
  `runMigrate(ex executor.Executor, cwd string, dryRun bool) (exitCode int, err error)`.

**Behavior contract (from spec):**

- Not a meta repo → return error `Not in a gogo-meta repository. Run "gogo init" first.` (printed by main, exit 1 —
  matches JS throw).
- Empty projects, or nothing to do → `Success("Working copy already matches configuration")`, exit 0.
- Conflicts → per-path `ProjectStatus(..,"error",..)` then return error `Migration aborted: ...`.
- Moves → rename, prune empty parents, update `.gitignore`, `ProjectStatus(to,"success","moved from <from>")`. Dry-run
  prints `Would move <from> → <to>`.
- Ambiguous / missing → warnings; non-dry-run with any missing/ambiguous returns `(1, nil)` so the action calls
  `os.Exit(1)` with no extra error line (matches JS `process.exitCode = 1`).
- Dry-run → `Info("Dry run: N move(s) pending")`, exit 0.

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/migrate_test.go` (package `cli`). Includes a cwd-keyed fake executor and the nine JS cases:

```go
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExecutor answers `git remote get-url origin` per directory.
type fakeExecutor struct{ remotes map[string]string } // abs dir -> url ("" => no remote)

func (f *fakeExecutor) Execute(_ context.Context, _ string, opts executor.Options) (*executor.Result, error) {
	url, ok := f.remotes[opts.Cwd]
	if !ok || url == "" {
		return &executor.Result{ExitCode: 1, Stderr: "no such remote"}, nil
	}
	return &executor.Result{ExitCode: 0, Stdout: url + "\n"}, nil
}

func writeGogo(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte(body), 0o644))
}

func mkGitRepo(t *testing.T, root, rel string) string {
	t.Helper()
	abs := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Join(abs, ".git"), 0o755))
	return abs
}

func captureOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	t.Cleanup(func() { output.Writer, output.ErrWriter = oldW, oldE })
	return &buf
}

func TestMigrateNotARepo(t *testing.T) {
	dir := t.TempDir()
	_ = captureOutput(t)
	_, err := runMigrate(&fakeExecutor{}, dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not in a gogo-meta repository")
}

func TestMigrateAlreadyInSync(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"api":"git@x:org/api.git"}}`)
	abs := mkGitRepo(t, dir, "api")
	buf := captureOutput(t)
	code, err := runMigrate(&fakeExecutor{remotes: map[string]string{abs: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Contains(t, buf.String(), "already matches")
	assert.DirExists(t, abs)
}

func TestMigrateMovesRepo(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	buf := captureOutput(t)
	code, err := runMigrate(&fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.DirExists(t, filepath.Join(dir, "packages/api/.git"))
	assert.NoDirExists(t, filepath.Join(dir, "lib/api"))
	assert.Contains(t, buf.String(), "lib/api")
}

func TestMigratePrunesEmptyParent(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	_ = captureOutput(t)
	_, err := runMigrate(&fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(dir, "lib"))
}

func TestMigrateKeepsNonEmptyParent(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	other := mkGitRepo(t, dir, "lib/web")
	_ = captureOutput(t)
	_, err := runMigrate(&fakeExecutor{remotes: map[string]string{
		from:  "git@x:org/api.git",
		other: "git@x:org/web.git",
	}}, dir, false)
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "packages/api/.git"))
	assert.DirExists(t, filepath.Join(dir, "lib/web/.git"))
	assert.DirExists(t, filepath.Join(dir, "lib"))
}

func TestMigrateUpdatesGitignore(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("lib/api\n"), 0o644))
	_ = captureOutput(t)
	_, err := runMigrate(&fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.NotContains(t, string(gi), "lib/api")
	assert.Contains(t, string(gi), "packages/api")
}

func TestMigrateDryRun(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	buf := captureOutput(t)
	code, err := runMigrate(&fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.DirExists(t, filepath.Join(dir, "lib/api"))
	assert.NoDirExists(t, filepath.Join(dir, "packages/api"))
	assert.Contains(t, buf.String(), "packages/api")
}

func TestMigrateConflictAborts(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"api":"git@x:org/api.git"}}`)
	target := mkGitRepo(t, dir, "api")
	buf := captureOutput(t)
	_, err := runMigrate(&fakeExecutor{remotes: map[string]string{target: "git@x:org/OTHER.git"}}, dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Migration aborted")
	assert.Contains(t, buf.String(), "occupied")
}

func TestMigrateMissingExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"api":"git@x:org/api.git"}}`)
	buf := captureOutput(t)
	code, err := runMigrate(&fakeExecutor{}, dir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, code)
	assert.Contains(t, buf.String(), "gogo git update")
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/cli/ -run TestMigrate -v`
Expected: FAIL — `runMigrate` undefined.

- [ ] **Step 3: Implement `internal/cli/migrate.go`**

```go
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/discover"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

const (
	notAGogoRepoMsg     = `Not in a gogo-meta repository. Run "gogo init" first.`
	migrationAbortedMsg = "Migration aborted: one or more target paths are occupied by a different repository"
)

type migrateMove struct{ from, to string }
type migrateConflict struct {
	path  string
	found string // "" means no remote
}

func newMigrateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Move/rename working-copy directories to match the configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			code, err := runMigrate(executor.NewShellExecutor(), cwd, dryRun)
			if err != nil {
				return err
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be moved without changing anything")
	return cmd
}

func getRemoteURL(ex executor.Executor, dir string) string {
	res, err := ex.Execute(context.Background(), "git remote get-url origin", executor.Options{Cwd: dir})
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func mapURLsToCurrentPaths(ex executor.Executor, metaDir string, ignore []string) (map[string]string, map[string]bool, error) {
	repoPaths, err := discover.FindGitRepos(metaDir, ignore)
	if err != nil {
		return nil, nil, err
	}
	urlToPath := map[string]string{}
	ambiguous := map[string]bool{}
	for _, rp := range repoPaths {
		url := getRemoteURL(ex, filepath.Join(metaDir, rp))
		if url == "" {
			continue
		}
		if _, ok := urlToPath[url]; ok {
			ambiguous[url] = true
			continue
		}
		urlToPath[url] = rp
	}
	return urlToPath, ambiguous, nil
}

func pruneEmptyParents(metaDir, movedFrom string) {
	root, err := filepath.Abs(metaDir)
	if err != nil {
		return
	}
	dir, err := filepath.Abs(filepath.Dir(filepath.Join(metaDir, movedFrom)))
	if err != nil {
		return
	}
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func runMigrate(ex executor.Executor, cwd string, dryRun bool) (int, error) {
	metaDir, err := config.GetMetaDir(cwd)
	if err != nil {
		return 0, err
	}
	if metaDir == "" {
		return 0, errors.New(notAGogoRepoMsg)
	}

	result, err := config.ReadMetaConfig(cwd, nil)
	if err != nil {
		return 0, err
	}
	cfg := result.Config

	if len(cfg.Projects) == 0 {
		output.Success("Working copy already matches configuration")
		return 0, nil
	}

	urlToPath, ambiguousURLs, err := mapURLsToCurrentPaths(ex, metaDir, cfg.Ignore)
	if err != nil {
		return 0, err
	}

	desired := make([]string, 0, len(cfg.Projects))
	for p := range cfg.Projects {
		desired = append(desired, p)
	}
	sort.Strings(desired)

	var moves []migrateMove
	var missing, ambiguous []string
	var conflicts []migrateConflict

	for _, projectPath := range desired {
		url := cfg.Projects[projectPath]
		targetDir := filepath.Join(metaDir, projectPath)

		if config.FileExists(targetDir) {
			targetRemote := getRemoteURL(ex, targetDir)
			if targetRemote != url {
				conflicts = append(conflicts, migrateConflict{path: projectPath, found: targetRemote})
			}
			continue
		}

		switch {
		case ambiguousURLs[url]:
			ambiguous = append(ambiguous, projectPath)
		case urlToPath[url] != "" && urlToPath[url] != projectPath:
			moves = append(moves, migrateMove{from: urlToPath[url], to: projectPath})
		default:
			missing = append(missing, projectPath)
		}
	}

	if len(conflicts) > 0 {
		for _, c := range conflicts {
			found := c.found
			if found == "" {
				found = "no remote"
			}
			output.ProjectStatus(c.path, "error", fmt.Sprintf("occupied by a different repository (found %s)", found))
		}
		return 0, errors.New(migrationAbortedMsg)
	}

	if len(moves) == 0 && len(missing) == 0 && len(ambiguous) == 0 {
		output.Success("Working copy already matches configuration")
		return 0, nil
	}

	for _, m := range moves {
		if dryRun {
			output.Info(fmt.Sprintf("Would move %s → %s", m.from, m.to))
			continue
		}
		targetDir := filepath.Join(metaDir, m.to)
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
			return 0, err
		}
		if err := os.Rename(filepath.Join(metaDir, m.from), targetDir); err != nil {
			return 0, err
		}
		pruneEmptyParents(metaDir, m.from)
		if _, err := config.RemoveFromGitignore(metaDir, m.from); err != nil {
			return 0, err
		}
		if _, err := config.AddToGitignore(metaDir, m.to); err != nil {
			return 0, err
		}
		output.ProjectStatus(m.to, "success", fmt.Sprintf("moved from %s", m.from))
	}

	for _, p := range ambiguous {
		output.Warning(fmt.Sprintf("%s: multiple working-copy directories share its repository URL — resolve manually", p))
	}
	for _, p := range missing {
		output.Warning(fmt.Sprintf("%s not found in working copy — run 'gogo git update' to clone", p))
	}

	if dryRun {
		output.Info(fmt.Sprintf("Dry run: %d move(s) pending", len(moves)))
		return 0, nil
	}

	if len(missing) > 0 || len(ambiguous) > 0 {
		return 1, nil
	}
	return 0, nil
}
```

Note on ST1005: `notAGogoRepoMsg` / `migrationAbortedMsg` are consts passed to `errors.New`, so the capitalized text is
preserved for parity without tripping the linter on a string literal.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/cli/ -run TestMigrate -v`
Expected: PASS (all nine).

- [ ] **Step 5: Lint + full test**

Run: `make lint && make test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/migrate.go internal/cli/migrate_test.go
git commit -m "$(cat <<'EOF'
feat(go,migrate): add migrate command to reconcile working copy

Mirror JS 8e8c05f + f2522b7: move/rename repos to match config, prune
empty parents, sync .gitignore, dry-run, conflict/missing/ambiguous handling.

Issue: gogo-meta-rewrite

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Register `migrate` + update docs

**Files:**

- Modify: `internal/cli/root.go` (add `newMigrateCmd()` to `AddCommand`)
- Modify: `README.md` (line 10 commit hash; remove `.looprc` docs; document `migrate`)
- Modify: `CLAUDE.md` (line 3 commit hash ×2; remove `.looprc` from Configuration Files; add `migrate` to command list +
  CLI usage)

**Interfaces:**

- Consumes: `newMigrateCmd()` from Task 5.
- Produces: `gogo migrate` reachable from the CLI; docs match behavior.

- [ ] **Step 1: Register the command**

In `internal/cli/root.go`, add `newMigrateCmd(),` to the `rootCmd.AddCommand(...)` list (place after
`newValidateCmd(),`).

- [ ] **Step 2: Verify it is wired**

Run: `go run ./cmd/gogo migrate --help`
Expected: help text showing `Move/rename working-copy directories to match the configuration` and the `--dry-run` flag.

- [ ] **Step 3: Update `README.md`**

- Line 10: replace the tree-URL hash `6ae349afce42af1081c6c40d64a0affb708ff562` with
  `44344a19bfc70995b142f49a51316dbe126e9f8f`.
- Remove any `.looprc` documentation section.
- Add a `gogo migrate` entry to the command list/usage (description: "Move/rename working-copy directories to match the
  configuration", with `--dry-run`).

Find the spots:

```bash
grep -n '6ae349a\|looprc\|.looprc' README.md
```

- [ ] **Step 4: Update `CLAUDE.md`**

- Line 3: replace BOTH occurrences of `6ae349afce42af1081c6c40d64a0affb708ff562` with
  `44344a19bfc70995b142f49a51316dbe126e9f8f`.
- In "Configuration Files", remove the `.looprc (optional filtering)` subsection.
- In the project structure / commands sections, add `migrate.go` and the `gogo migrate` usage line.

```bash
grep -n '6ae349a\|looprc\|.looprc' CLAUDE.md
```

Expected after edits: no `looprc` hits; no `6ae349a` hits.

- [ ] **Step 5: Final verification**

Run: `make lint && make test`
Expected: PASS.

```bash
grep -rn 'looprc\|6ae349a' README.md CLAUDE.md internal/ cmd/ && echo "RESIDUE" || echo "clean"
```

Expected: `clean`.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/root.go README.md CLAUDE.md
git commit -m "$(cat <<'EOF'
docs(go): register migrate, refresh parity baseline, drop .looprc docs

Wire migrate into root cmd; bump reimplementation commit hash to JS main
HEAD in README/CLAUDE; remove .looprc documentation.

Issue: gogo-meta-rewrite

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Final parity sweep (after all tasks)

- [ ] `make all` (clean, lint, test-coverage, build) is green.
- [ ] `grep -rn 'looprc' .` finds nothing outside `docs/superpowers/` and `CHANGELOG`-style history.
- [ ] Manual spot-check: in a scratch meta repo, `gogo migrate --dry-run`, `gogo migrate`, and `gogo validate` produce
  output matching the JS tool case-for-case (move message, "already matches", missing-dir hint, conflict abort).
- [ ] Tag the completed stage: `git tag -a gogo-meta-rewrite-parity -m "Go reaches parity with JS main 44344a1"`.
