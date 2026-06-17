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
