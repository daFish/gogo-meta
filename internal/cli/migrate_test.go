package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
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

func (f *fakeExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts executor.Options) (*executor.Result, error) {
	return f.Execute(ctx, strings.TrimSpace(name+" "+strings.Join(args, " ")), opts)
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
	_, err := runMigrate(context.Background(), &fakeExecutor{}, dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not in a gogo-meta repository")
}

func TestMigrateAlreadyInSync(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"api":"git@x:org/api.git"}}`)
	abs := mkGitRepo(t, dir, "api")
	buf := captureOutput(t)
	code, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{abs: "git@x:org/api.git"}}, dir, false)
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
	code, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.DirExists(t, filepath.Join(dir, "packages", "api", ".git"))
	assert.NoDirExists(t, filepath.Join(dir, "lib", "api"))
	assert.Contains(t, buf.String(), "lib/api")
}

func TestMigratePrunesEmptyParent(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	_ = captureOutput(t)
	_, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(dir, "lib"))
}

func TestMigrateKeepsNonEmptyParent(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	other := mkGitRepo(t, dir, "lib/web")
	_ = captureOutput(t)
	_, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{
		from:  "git@x:org/api.git",
		other: "git@x:org/web.git",
	}}, dir, false)
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "packages", "api", ".git"))
	assert.DirExists(t, filepath.Join(dir, "lib", "web", ".git"))
	assert.DirExists(t, filepath.Join(dir, "lib"))
}

func TestMigrateUpdatesGitignore(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("lib/api\n"), 0o644))
	_ = captureOutput(t)
	_, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.NotContains(t, string(gi), "lib/api")
	assert.Contains(t, string(gi), "packages/api")
}

func TestMigrateLocalProjectMoveGoesToGitExclude(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	writeGogo(t, dir, `{"projects":{}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{"packages/api":"git@x:org/api.git"}}`), 0o644))
	from := mkGitRepo(t, dir, "lib/api")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("lib/api\n"), 0o644))
	config.SetOverlayFiles(nil)
	_ = captureOutput(t)

	code, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.DirExists(t, filepath.Join(dir, "packages", "api", ".git"))

	excl, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	require.NoError(t, err)
	assert.Contains(t, string(excl), "packages/api", "local move → .git/info/exclude")

	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.NotContains(t, string(gi), "packages/api", "local move must not touch shared .gitignore")
}

func TestMigrateDryRun(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	buf := captureOutput(t)
	code, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.DirExists(t, filepath.Join(dir, "lib", "api"))
	assert.NoDirExists(t, filepath.Join(dir, "packages", "api"))
	assert.Contains(t, buf.String(), "packages/api")
}

func TestMigrateConflictAborts(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"api":"git@x:org/api.git"}}`)
	target := mkGitRepo(t, dir, "api")
	buf := captureOutput(t)
	_, err := runMigrate(context.Background(), &fakeExecutor{remotes: map[string]string{target: "git@x:org/OTHER.git"}}, dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Migration aborted")
	assert.Contains(t, buf.String(), "occupied")
}

func TestMigrateMissingExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"api":"git@x:org/api.git"}}`)
	buf := captureOutput(t)
	code, err := runMigrate(context.Background(), &fakeExecutor{}, dir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, code)
	assert.Contains(t, buf.String(), "gogo git update")
}

func TestMigrateDryRunMatchesRealRunExitCode(t *testing.T) {
	tests := []struct {
		name     string
		gogo     string
		repos    map[string]string
		wantCode int
	}{
		{
			name:     "missing project",
			gogo:     `{"projects":{"api":"git@x:org/api.git"}}`,
			wantCode: 1,
		},
		{
			name:     "everything already in place",
			gogo:     `{"projects":{}}`,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, dryRun := range []bool{false, true} {
				dir := t.TempDir()
				writeGogo(t, dir, tt.gogo)
				_ = captureOutput(t)

				code, err := runMigrate(context.Background(), &fakeExecutor{remotes: tt.repos}, dir, dryRun)
				require.NoError(t, err)
				assert.Equal(t, tt.wantCode, code,
					"dry run and real run must report the same exit code (dryRun=%v)", dryRun)
			}
		})
	}
}

func TestMigrateDryRunChangesNothingWhileReportingFailure(t *testing.T) {
	dir := t.TempDir()
	writeGogo(t, dir, `{"projects":{"packages/api":"git@x:org/api.git","web":"git@x:org/web.git"}}`)
	from := mkGitRepo(t, dir, "lib/api")
	buf := captureOutput(t)

	code, err := runMigrate(context.Background(),
		&fakeExecutor{remotes: map[string]string{from: "git@x:org/api.git"}}, dir, true)

	require.NoError(t, err)
	assert.Equal(t, 1, code, "a pending move plus a missing project is a failure the dry run must report")
	assert.DirExists(t, filepath.Join(dir, "lib", "api"), "dry run must not move anything")
	assert.NoDirExists(t, filepath.Join(dir, "packages", "api"))
	assert.Contains(t, buf.String(), "Dry run")
}

func TestPlanMigrationBuckets(t *testing.T) {
	dir := t.TempDir()

	// present: configured path exists with the configured origin
	present := mkGitRepo(t, dir, "apps/web")
	// conflict: configured path exists with a different origin
	conflict := mkGitRepo(t, dir, "apps/api")
	// move: the configured path is empty, the repo sits elsewhere
	moved := mkGitRepo(t, dir, "old/shared")
	// ambiguous: two checkouts share the URL the config wants
	dup1 := mkGitRepo(t, dir, "one/tools")
	dup2 := mkGitRepo(t, dir, "two/tools")

	cfg := config.MetaConfig{Projects: map[string]string{
		"apps/web":     "git@x:o/web.git",
		"apps/api":     "git@x:o/api.git",
		"libs/shared":  "git@x:o/shared.git",
		"libs/tools":   "git@x:o/tools.git",
		"libs/nowhere": "git@x:o/nowhere.git",
	}}

	ex := &fakeExecutor{remotes: map[string]string{
		present:  "git@x:o/web.git",
		conflict: "git@x:o/other.git",
		moved:    "git@x:o/shared.git",
		dup1:     "git@x:o/tools.git",
		dup2:     "git@x:o/tools.git",
	}}

	selected := []string{"apps/api", "apps/web", "libs/nowhere", "libs/shared", "libs/tools"}
	plan, err := planMigration(context.Background(), ex, dir, cfg, selected)
	require.NoError(t, err)

	assert.Equal(t, []string{"apps/web"}, plan.present)
	assert.Equal(t, []string{"libs/nowhere"}, plan.missing)
	assert.Equal(t, []string{"libs/tools"}, plan.ambiguous)
	require.Len(t, plan.moves, 1)
	assert.Equal(t, "old/shared", plan.moves[0].from)
	assert.Equal(t, "libs/shared", plan.moves[0].to)
	require.Len(t, plan.conflicts, 1)
	assert.Equal(t, "apps/api", plan.conflicts[0].path)
	assert.Equal(t, "git@x:o/other.git", plan.conflicts[0].found)

	t.Run("every selected project lands in exactly one bucket", func(t *testing.T) {
		seen := map[string]int{}
		for _, p := range plan.present {
			seen[p]++
		}
		for _, p := range plan.missing {
			seen[p]++
		}
		for _, p := range plan.ambiguous {
			seen[p]++
		}
		for _, m := range plan.moves {
			seen[m.to]++
		}
		for _, c := range plan.conflicts {
			seen[c.path]++
		}
		for _, p := range selected {
			assert.Equal(t, 1, seen[p], "%s must appear in exactly one bucket", p)
		}
	})
}

func TestApplyMovesReportsCompletedMoves(t *testing.T) {
	dir := t.TempDir()
	mkGitRepo(t, dir, "old/api")
	mkGitRepo(t, dir, "old/web")

	moves := []migrateMove{
		{from: "old/api", to: "apps/api"},
		{from: "old/web", to: "apps/web"},
	}

	applied, err := applyMoves(dir, moves, map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, moves, applied)
	assert.DirExists(t, filepath.Join(dir, "apps", "api"))
	assert.DirExists(t, filepath.Join(dir, "apps", "web"))
	assert.NoDirExists(t, filepath.Join(dir, "old"), "emptied parents are pruned")

	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(gitignore), "apps/api")
	assert.NotContains(t, string(gitignore), "old/api")
}
