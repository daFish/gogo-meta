package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"),
		[]byte("# my stuff\n*.log\n"), 0o644))

	_, err := SyncGitExcludeManagedBlock(dir, []string{"personal"})
	require.NoError(t, err)
	got := readExclude(t, dir)
	assert.Contains(t, got, "*.log", "user content preserved")
	assert.Contains(t, got, "personal")

	changed, err := SyncGitExcludeManagedBlock(dir, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	got = readExclude(t, dir)
	assert.Contains(t, got, "*.log")
	assert.NotContains(t, got, "personal")
	assert.NotContains(t, got, gitExcludeManagedHeader)
}

func TestSyncGitExcludeManagedBlockNoRepo(t *testing.T) {
	dir := t.TempDir()
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

func TestSyncGitExcludeManagedBlockRefusesHeaderWithoutFooter(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	broken := gitExcludeManagedHeader + "\nold-entry\n# user note below broken block\n*.log\n"
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	require.NoError(t, os.WriteFile(excludePath, []byte(broken), 0o644))

	changed, err := SyncGitExcludeManagedBlock(dir, []string{"personal"})
	require.Error(t, err, "header without footer must be an error, not a guess")
	assert.Contains(t, err.Error(), "without its footer")
	assert.False(t, changed)

	got := readExclude(t, dir)
	assert.Equal(t, broken, got, "file must be left byte-for-byte untouched")
}
