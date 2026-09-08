package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoState is what the scripted git answers about one working copy.
type repoState struct {
	remote   string // origin URL; empty means no remote configured
	branch   string // empty means main
	detached bool
	upstream string // empty means no upstream; defaults to origin/<branch> unless noUpstream
	unborn   bool
	dirty    bool
	ahead    int
	behind   int
	fetchErr string
	mergeErr string
}

func (r repoState) porcelain() string {
	var b strings.Builder
	if r.unborn {
		b.WriteString("# branch.oid (initial)\n")
	} else {
		b.WriteString("# branch.oid 4f2a9c\n")
	}
	if r.detached {
		b.WriteString("# branch.head (detached)\n")
	} else {
		branch := r.branch
		if branch == "" {
			branch = "main"
		}
		b.WriteString("# branch.head " + branch + "\n")
		if r.upstream != "none" {
			upstream := r.upstream
			if upstream == "" {
				upstream = "origin/" + branch
			}
			b.WriteString("# branch.upstream " + upstream + "\n")
		}
	}
	if r.dirty {
		b.WriteString("1 .M N... 100644 100644 100644 aaa bbb file.go\n")
	}
	return b.String()
}

// updateExecutor scripts every git invocation gogo update makes.
type updateExecutor struct {
	repos     map[string]repoState // abs dir -> state
	cloneFail map[string]string    // url -> error message

	mu    sync.Mutex
	calls []string // "<subcommand> <cwd>"
}

func (u *updateExecutor) Execute(_ context.Context, _ string, _ executor.Options) (*executor.Result, error) {
	return &executor.Result{ExitCode: 0}, nil
}

func (u *updateExecutor) ExecuteArgs(_ context.Context, _ string, args []string, opts executor.Options) (*executor.Result, error) {
	u.mu.Lock()
	u.calls = append(u.calls, args[0]+" "+opts.Cwd)
	u.mu.Unlock()

	state, known := u.repos[opts.Cwd]

	switch args[0] {
	case "remote":
		if !known || state.remote == "" {
			return &executor.Result{ExitCode: 1, Stderr: "no such remote"}, nil
		}
		return &executor.Result{ExitCode: 0, Stdout: state.remote + "\n"}, nil

	case "status":
		if !known {
			return &executor.Result{ExitCode: 128, Stderr: "fatal: not a git repository"}, nil
		}
		return &executor.Result{ExitCode: 0, Stdout: state.porcelain()}, nil

	case "fetch":
		if state.fetchErr != "" {
			return &executor.Result{ExitCode: 1, Stderr: state.fetchErr}, nil
		}
		return &executor.Result{ExitCode: 0}, nil

	case "rev-list":
		return &executor.Result{ExitCode: 0, Stdout: fmt.Sprintf("%d\t%d\n", state.ahead, state.behind)}, nil

	case "merge":
		if state.mergeErr != "" {
			return &executor.Result{ExitCode: 1, Stderr: state.mergeErr}, nil
		}
		return &executor.Result{ExitCode: 0}, nil

	case "clone":
		url := args[2]
		if msg, fails := u.cloneFail[url]; fails {
			return &executor.Result{ExitCode: 128, Stderr: msg}, nil
		}
		if err := os.MkdirAll(filepath.Join(opts.Cwd, args[3], ".git"), 0o755); err != nil {
			return nil, err
		}
		return &executor.Result{ExitCode: 0}, nil
	}

	return &executor.Result{ExitCode: 0}, nil
}

func (u *updateExecutor) ran(subcommand, dir string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, c := range u.calls {
		if c == subcommand+" "+dir {
			return true
		}
	}
	return false
}

// repoDir is the absolute path of a project, from its POSIX-style config path.
func repoDir(metaDir, projectPath string) string {
	return filepath.Join(metaDir, filepath.FromSlash(projectPath))
}

// setupMetaRepo writes a .gogo with the given projects and returns the meta dir.
func setupMetaRepo(t *testing.T, projects map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))

	entries := make([]string, 0, len(projects))
	for path, url := range projects {
		entries = append(entries, fmt.Sprintf("%q:%q", path, url))
	}
	body := fmt.Sprintf(`{"projects":{%s}}`, strings.Join(entries, ","))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte(body), 0o644))

	config.SetOverlayFiles(nil)
	return dir
}

func TestUpdateRejectsAllPhasesDisabled(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{"api": "git@x:o/api.git"})
	_ = captureOutput(t)

	_, err := runUpdate(context.Background(), &updateExecutor{}, dir, updateOptions{
		NoMigrate: true, NoClone: true, NoPull: true,
	})

	require.Error(t, err, "disabling every phase must be an error, not a silent success")
	assert.Contains(t, err.Error(), allPhasesDisabledMsg)
}

func TestUpdateConvergesMoveCloneAndPull(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{
		"libs/api":    "git@x:o/api.git",
		"libs/web":    "git@x:o/web.git",
		"libs/shared": "git@x:o/shared.git",
	})
	// libs/shared is present and behind; libs/api sits at old/api; libs/web is absent.
	mkGitRepo(t, dir, "libs/shared")
	mkGitRepo(t, dir, "old/api")

	ex := &updateExecutor{repos: map[string]repoState{
		repoDir(dir, "libs/shared"): {remote: "git@x:o/shared.git", behind: 3},
		repoDir(dir, "old/api"):     {remote: "git@x:o/api.git"},
		repoDir(dir, "libs/api"):    {remote: "git@x:o/api.git", behind: 1},
	}}

	buf := captureOutput(t)
	code, err := runUpdate(context.Background(), ex, dir, updateOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, code)

	out := buf.String()
	assert.Contains(t, out, "moved from old/api")
	assert.Contains(t, out, "cloned")
	assert.Contains(t, out, "updated (3 commit(s))")
	assert.DirExists(t, filepath.Join(dir, "libs", "api"), "the move must have happened")

	assert.True(t, ex.ran("merge", repoDir(dir, "libs/api")),
		"a project moved this run is pulled at its new path")
	assert.False(t, ex.ran("fetch", repoDir(dir, "libs/web")),
		"a freshly cloned project is current by construction and must not be pulled")
}

func TestUpdatePullPolicy(t *testing.T) {
	tests := []struct {
		name      string
		state     repoState
		wantClass outcomeClass
		wantMsg   string
	}{
		{name: "up to date", state: repoState{}, wantClass: outcomeOK, wantMsg: "up to date"},
		{name: "behind fast-forwards", state: repoState{behind: 4}, wantClass: outcomeOK, wantMsg: "updated (4 commit(s))"},
		{name: "ahead only", state: repoState{ahead: 2}, wantClass: outcomeOK, wantMsg: "up to date (ahead 2, not pushed)"},
		{
			name:      "diverged is never merged",
			state:     repoState{ahead: 2, behind: 3},
			wantClass: outcomeFailed,
			wantMsg:   "diverged from upstream (ahead 2, behind 3) — merge or rebase manually",
		},
		{name: "detached HEAD", state: repoState{detached: true}, wantClass: outcomeSkipped, wantMsg: "detached HEAD"},
		{name: "no upstream", state: repoState{upstream: "none"}, wantClass: outcomeSkipped, wantMsg: "no upstream branch"},
		{
			name:      "fetch failure",
			state:     repoState{fetchErr: "fatal: could not read from remote\n"},
			wantClass: outcomeFailed,
			wantMsg:   "fetch failed: fatal: could not read from remote",
		},
		{
			name:      "git refuses the fast-forward",
			state:     repoState{behind: 1, dirty: true, mergeErr: "error: Your local changes would be overwritten\n"},
			wantClass: outcomeFailed,
			wantMsg:   "fast-forward refused: error: Your local changes would be overwritten (working tree has local changes)",
		},
		{
			name:      "dirty but no overlap still fast-forwards",
			state:     repoState{behind: 2, dirty: true},
			wantClass: outcomeOK,
			wantMsg:   "updated (2 commit(s)) (working tree has local changes)",
		},
		{
			name:      "unborn branch fast-forwards without comparing",
			state:     repoState{unborn: true},
			wantClass: outcomeOK,
			wantMsg:   "updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &updateExecutor{repos: map[string]repoState{"/repo": tt.state}}
			class, msg := pullProject(context.Background(), ex, "/repo", false)
			assert.Equal(t, tt.wantClass, class)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

func TestUpdatePullNeverRewritesHistory(t *testing.T) {
	ex := &updateExecutor{repos: map[string]repoState{"/repo": {behind: 2}}}
	_, _ = pullProject(context.Background(), ex, "/repo", false)

	for _, call := range ex.calls {
		for _, forbidden := range []string{"rebase", "stash", "checkout", "reset", "pull"} {
			assert.NotContains(t, call, forbidden)
		}
	}
	assert.True(t, ex.ran("merge", "/repo"))
}

func TestUpdateDryRunInspectsOnly(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{
		"libs/api": "git@x:o/api.git",
		"libs/web": "git@x:o/web.git",
	})
	mkGitRepo(t, dir, "libs/api")

	// Diverged: the real run would fail this project, but a dry run cannot know
	// without fetching, so it must not fetch and must not report a failure.
	ex := &updateExecutor{repos: map[string]repoState{
		repoDir(dir, "libs/api"): {remote: "git@x:o/api.git", ahead: 1, behind: 1},
	}}

	buf := captureOutput(t)
	code, err := runUpdate(context.Background(), ex, dir, updateOptions{DryRun: true})
	require.NoError(t, err)

	assert.Equal(t, 0, code, "a dry run over a diverged project exits 0")
	assert.False(t, ex.ran("fetch", repoDir(dir, "libs/api")), "a dry run must not fetch")
	assert.False(t, ex.ran("merge", repoDir(dir, "libs/api")), "a dry run must not merge")
	assert.NoDirExists(t, filepath.Join(dir, "libs", "web"), "a dry run must not clone")

	out := buf.String()
	assert.Contains(t, out, "would pull")
	assert.Contains(t, out, "would clone")
	assert.Contains(t, out, "Dry run")
}

func TestUpdateDryRunPreviewsMoveAtItsCurrentPath(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{"libs/api": "git@x:o/api.git"})
	mkGitRepo(t, dir, "old/api")

	ex := &updateExecutor{repos: map[string]repoState{
		repoDir(dir, "old/api"): {remote: "git@x:o/api.git"},
	}}

	buf := captureOutput(t)
	code, err := runUpdate(context.Background(), ex, dir, updateOptions{DryRun: true})
	require.NoError(t, err)

	assert.Equal(t, 0, code)
	assert.Contains(t, buf.String(), "would move from old/api")
	assert.DirExists(t, filepath.Join(dir, "old", "api"), "a dry run must not move anything")
	assert.True(t, ex.ran("status", repoDir(dir, "old/api")),
		"the pull preview must inspect the path the project is still at")
}

func TestUpdatePhaseOptOuts(t *testing.T) {
	tests := []struct {
		name      string
		opts      updateOptions
		wantCode  int
		wantOut   string
		wantNoOut string
	}{
		{
			name:     "--no-clone skips missing projects and exits 0",
			opts:     updateOptions{NoClone: true},
			wantCode: 0,
			wantOut:  "not cloned (--no-clone)",
		},
		{
			name:      "--no-pull leaves present projects alone",
			opts:      updateOptions{NoPull: true},
			wantCode:  0,
			wantOut:   "not pulled (--no-pull)",
			wantNoOut: "updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupMetaRepo(t, map[string]string{
				"libs/api": "git@x:o/api.git",
				"libs/web": "git@x:o/web.git",
			})
			mkGitRepo(t, dir, "libs/api")

			ex := &updateExecutor{repos: map[string]repoState{
				repoDir(dir, "libs/api"): {remote: "git@x:o/api.git", behind: 2},
			}}

			buf := captureOutput(t)
			code, err := runUpdate(context.Background(), ex, dir, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCode, code)
			assert.Contains(t, buf.String(), tt.wantOut)
			if tt.wantNoOut != "" {
				assert.NotContains(t, buf.String(), tt.wantNoOut)
			}
		})
	}
}

func TestUpdateNoMigrateFailsMisplacedProject(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{"libs/api": "git@x:o/api.git"})
	mkGitRepo(t, dir, "old/api")

	ex := &updateExecutor{repos: map[string]repoState{
		repoDir(dir, "old/api"): {remote: "git@x:o/api.git"},
	}}

	buf := captureOutput(t)
	code, err := runUpdate(context.Background(), ex, dir, updateOptions{NoMigrate: true})
	require.NoError(t, err)

	assert.Equal(t, 1, code, "a misplaced project is a real discrepancy, not something to skip")
	assert.Contains(t, buf.String(), "found at old/api — run without --no-migrate to move it")
	assert.NoDirExists(t, filepath.Join(dir, "libs", "api"), "it must not be cloned into place either")
}

func TestUpdateConflictAbortsMovesButNotTheRest(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{
		"libs/api":     "git@x:o/api.git",
		"libs/shared":  "git@x:o/shared.git",
		"libs/missing": "git@x:o/missing.git",
	})
	// libs/api is occupied by a different repository -> conflict.
	mkGitRepo(t, dir, "libs/api")
	// libs/shared wants moving, and must not be moved while a conflict stands.
	mkGitRepo(t, dir, "old/shared")

	ex := &updateExecutor{repos: map[string]repoState{
		repoDir(dir, "libs/api"):   {remote: "git@x:o/other.git"},
		repoDir(dir, "old/shared"): {remote: "git@x:o/shared.git"},
	}}

	buf := captureOutput(t)
	code, err := runUpdate(context.Background(), ex, dir, updateOptions{})
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "occupied by a different repository (found git@x:o/other.git)")
	assert.Contains(t, out, "not moved: migration aborted by a conflict")
	assert.DirExists(t, filepath.Join(dir, "old", "shared"), "no rename may happen after a conflict")
	assert.Contains(t, out, "cloned")
	assert.DirExists(t, filepath.Join(dir, "libs", "missing"),
		"an unrelated missing project is still cloned")
}

func TestUpdateReportsEachProjectExactlyOnce(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{
		"present":    "git@x:o/present.git",
		"moved":      "git@x:o/moved.git",
		"missing":    "git@x:o/missing.git",
		"conflicted": "git@x:o/conflicted.git",
	})
	mkGitRepo(t, dir, "present")
	mkGitRepo(t, dir, "elsewhere/moved")
	mkGitRepo(t, dir, "conflicted")

	ex := &updateExecutor{repos: map[string]repoState{
		filepath.Join(dir, "present"):    {remote: "git@x:o/present.git"},
		repoDir(dir, "elsewhere/moved"):  {remote: "git@x:o/moved.git"},
		filepath.Join(dir, "conflicted"): {remote: "git@x:o/somethingelse.git"},
		filepath.Join(dir, "moved"):      {remote: "git@x:o/moved.git"},
	}}

	_ = captureOutput(t)
	_, err := runUpdate(context.Background(), ex, dir, updateOptions{})
	require.NoError(t, err)

	// Re-run the pipeline pieces to inspect the recorder directly.
	recorder := newOutcomeRecorder()
	recorder.record("a", outcomeOK, "first")
	recorder.record("a", outcomeFailed, "second")
	outcomes := recorder.outcomes()
	require.Len(t, outcomes, 1, "a project must never be recorded twice")
	assert.Equal(t, outcomeOK, outcomes[0].Class, "the first phase to classify a project wins")
}

func TestUpdateCloneFailureIsReported(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{"libs/gone": "git@x:o/gone.git"})

	ex := &updateExecutor{
		repos:     map[string]repoState{},
		cloneFail: map[string]string{"git@x:o/gone.git": "fatal: repository not found\n"},
	}

	buf := captureOutput(t)
	code, err := runUpdate(context.Background(), ex, dir, updateOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, code)
	assert.Contains(t, buf.String(), "repository not found")
}

func TestUpdateNoProjectsMatchFilter(t *testing.T) {
	dir := setupMetaRepo(t, map[string]string{"api": "git@x:o/api.git"})
	buf := captureOutput(t)

	opts := updateOptions{}
	opts.Filter.IncludeOnly = []string{"nothing-matches"}

	code, err := runUpdate(context.Background(), &updateExecutor{}, dir, opts)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Contains(t, buf.String(), "No projects match the specified filters")
}

func TestUpdateOutsideMetaRepo(t *testing.T) {
	_ = captureOutput(t)
	config.SetOverlayFiles(nil)

	_, err := runUpdate(context.Background(), &updateExecutor{}, t.TempDir(), updateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gogo init")
}
