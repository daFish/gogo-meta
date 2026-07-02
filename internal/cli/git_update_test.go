package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
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
	// Both dirs already present → no cloning happens; exclude routing still runs.
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

	// Drop the project from .gogo.local and re-run — the stale entry must vanish.
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
