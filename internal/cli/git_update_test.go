package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitUpdateExcludesLocalProjectDirs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"shared":"git@x:o/shared.git"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{"personal":"git@x:o/personal.git"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "shared"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "personal"), 0o755))

	config.SetOverlayFiles(nil)
	_ = captureOutput(t)
	initTestChdir(t, dir)

	cmd := newGitUpdateCmd()
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.Execute())

	excl, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	require.NoError(t, err)
	assert.Contains(t, string(excl), "personal", "local project dir must be in .git/info/exclude")
	assert.NotContains(t, string(excl), "shared", "shared project dir must NOT be in exclude")

	if b, err := os.ReadFile(filepath.Join(dir, ".gitignore")); err == nil {
		assert.NotContains(t, string(b), "personal",
			"local project dir must NOT leak into shared .gitignore")
	}
}

func TestGitUpdatePrunesRemovedLocalProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"shared":"git@x:o/shared.git"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{"personal":"git@x:o/personal.git"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "shared"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "personal"), 0o755))
	config.SetOverlayFiles(nil)
	_ = captureOutput(t)
	initTestChdir(t, dir)

	require.NoError(t, newGitUpdateCmd().Execute())
	require.Contains(t, readExcludeFile(t, dir), "personal")

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{}}`), 0o644))
	require.NoError(t, newGitUpdateCmd().Execute())
	assert.NotContains(t, readExcludeFile(t, dir), "personal",
		"removing a project from .gogo.local must prune its .git/info/exclude entry")
}

func readExcludeFile(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	require.NoError(t, err)
	return string(b)
}

func TestGitUpdateRejectsPositionalArgs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"api":"git@x:o/api.git","web":"git@x:o/web.git"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "web"), 0o755))

	config.SetOverlayFiles(nil)
	_ = captureOutput(t)
	initTestChdir(t, dir)

	cmd := newGitUpdateCmd()
	cmd.SetArgs([]string{"web"})
	err := cmd.Execute()

	require.Error(t, err, "a positional argument must not be silently ignored")
	assert.Contains(t, err.Error(), "unknown command")
}

// TestGitUpdateParallelFlagPositions pins that --parallel and --concurrency
// reach the clone pool from either side of the subcommand. git update declares
// its own copies of both flags while the root declares them persistently, so
// the two could shadow each other.
func TestGitUpdateParallelFlagPositions(t *testing.T) {
	resolveVia := func(t *testing.T, args ...string) (bool, int) {
		t.Helper()
		root := NewRootCommand("test")
		updateCmd, _, err := root.Find([]string{"git", "update"})
		require.NoError(t, err)

		var parallel bool
		var concurrency int
		updateCmd.RunE = func(cmd *cobra.Command, _ []string) error {
			parallel = getBoolFlag(cmd, "parallel")
			concurrency = getIntFlag(cmd, "concurrency")
			return nil
		}
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		require.NoError(t, root.Execute())
		return parallel, concurrency
	}

	t.Run("before the subcommand", func(t *testing.T) {
		parallel, concurrency := resolveVia(t, "--parallel", "--concurrency", "8", "git", "update")
		assert.True(t, parallel, "--parallel before the subcommand must reach the clone pool")
		assert.Equal(t, 8, concurrency)
	})

	t.Run("after the subcommand", func(t *testing.T) {
		parallel, concurrency := resolveVia(t, "git", "update", "--parallel", "--concurrency", "3")
		assert.True(t, parallel, "--parallel after the subcommand must reach the clone pool")
		assert.Equal(t, 3, concurrency)
	})

	t.Run("absent", func(t *testing.T) {
		parallel, concurrency := resolveVia(t, "git", "update")
		assert.False(t, parallel)
		assert.Zero(t, concurrency, "cloneMissing falls back to loop.DefaultConcurrency")
	})
}
