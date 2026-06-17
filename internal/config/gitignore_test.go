package config

import (
	"os"
	"path/filepath"
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
