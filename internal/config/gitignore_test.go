package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddToGitignore(t *testing.T) {
	t.Run("creates .gitignore if not exists", func(t *testing.T) {
		dir := t.TempDir()

		added, err := AddToGitignore(dir, "api")
		require.NoError(t, err)
		assert.True(t, added)

		content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, "api\n", string(content))
	})

	t.Run("appends entry to existing .gitignore", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules\n")

		added, err := AddToGitignore(dir, "api")
		require.NoError(t, err)
		assert.True(t, added)

		content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, "node_modules\napi\n", string(content))
	})

	t.Run("returns false if entry already exists", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules\napi\n")

		added, err := AddToGitignore(dir, "api")
		require.NoError(t, err)
		assert.False(t, added)

		content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, "node_modules\napi\n", string(content))
	})

	t.Run("handles .gitignore without trailing newline", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules")

		added, err := AddToGitignore(dir, "api")
		require.NoError(t, err)
		assert.True(t, added)

		content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, "node_modules\napi\n", string(content))
	})

	t.Run("handles entry with surrounding whitespace", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gitignore"), "  api  \nnode_modules\n")

		added, err := AddToGitignore(dir, "api")
		require.NoError(t, err)
		assert.False(t, added)
	})
}

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

func readExclude(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	require.NoError(t, err)
	return string(b)
}

func TestSyncGitExcludeManagedBlockAddsSortedBlock(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))

	changed, err := SyncGitExcludeManagedBlock(dir, []string{"b/repo", "a/repo", "b/repo"})
	require.NoError(t, err)
	assert.True(t, changed)

	got := readExclude(t, dir)
	assert.Contains(t, got, gitExcludeManagedHeader)
	assert.Contains(t, got, gitExcludeManagedFooter)
	// Sorted + deduped.
	assert.Less(t, strings.Index(got, "a/repo"), strings.Index(got, "b/repo"))
	assert.Equal(t, 1, strings.Count(got, "b/repo\n"))
}

func TestSyncGitExcludeManagedBlockIdempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))

	_, err := SyncGitExcludeManagedBlock(dir, []string{"x"})
	require.NoError(t, err)
	changed, err := SyncGitExcludeManagedBlock(dir, []string{"x"})
	require.NoError(t, err)
	assert.False(t, changed, "second identical sync must not rewrite")
}

func TestSyncGitExcludeManagedBlockPreservesUserLinesAndPrunes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	// User's own manual exclude content.
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"),
		[]byte("# my stuff\n*.log\n"), 0o644))

	_, err := SyncGitExcludeManagedBlock(dir, []string{"personal"})
	require.NoError(t, err)
	got := readExclude(t, dir)
	assert.Contains(t, got, "*.log", "user content preserved")
	assert.Contains(t, got, "personal")

	// Dropping the project removes the managed block but keeps user content.
	changed, err := SyncGitExcludeManagedBlock(dir, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	got = readExclude(t, dir)
	assert.Contains(t, got, "*.log")
	assert.NotContains(t, got, "personal")
	assert.NotContains(t, got, gitExcludeManagedHeader)
}

func TestSyncGitExcludeManagedBlockNoRepo(t *testing.T) {
	dir := t.TempDir() // no .git
	changed, err := SyncGitExcludeManagedBlock(dir, []string{"foo"})
	require.NoError(t, err)
	assert.False(t, changed, "no .git dir → skip silently")
}

func TestSyncGitExcludeManagedBlockEmptyNoBlockIsNoop(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"),
		[]byte("# pristine\n"), 0o644))
	changed, err := SyncGitExcludeManagedBlock(dir, nil)
	require.NoError(t, err)
	assert.False(t, changed, "no entries and no existing block → leave file untouched")
}
