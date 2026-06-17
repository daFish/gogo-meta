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
	assert.DirExists(t, filepath.Join(dir, "packages", "api", ".git"))
	assert.NoDirExists(t, filepath.Join(dir, "lib", "api"))
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
	assert.DirExists(t, filepath.Join(dir, "lib", "api"))
	assert.NoDirExists(t, filepath.Join(dir, "packages", "api"))
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
