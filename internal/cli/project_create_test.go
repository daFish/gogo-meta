package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectCreateRejectsUnsafeFolder(t *testing.T) {
	err := runProjectCreate(nil, []string{"../evil", "git@x:o/r.git"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project folder")
}

func TestProjectCreateRejectsUnsafeURL(t *testing.T) {
	err := runProjectCreate(nil, []string{"safe-folder", "ext::sh -c 'touch /tmp/x'"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git URL")
}

func TestProjectCreateDoesNotPersistLocalOverlayProjects(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"shared":"git@example.com:org/shared.git"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{"personal":"git@example.com:me/personal.git"}}`), 0o644))

	_ = captureOutput(t)
	initTestChdir(t, dir)

	cmd := newProjectCreateCmd()
	cmd.SetArgs([]string{"newrepo", "git@example.com:org/newrepo.git"})
	require.NoError(t, cmd.Execute())

	raw, err := os.ReadFile(filepath.Join(dir, ".gogo"))
	require.NoError(t, err)
	written := string(raw)
	require.Contains(t, written, "shared")
	require.Contains(t, written, "newrepo")
	require.NotContains(t, written, "personal",
		"project create must not persist .gogo.local-only projects into the shared .gogo")
}
