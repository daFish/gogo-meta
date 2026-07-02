package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectImportEnsuresLocalConfigIgnored(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{}}`), 0o644))
	config.SetOverlayFiles(nil)
	_ = captureOutput(t)
	initTestChdir(t, dir)

	cmd := newProjectImportCmd()
	cmd.SetArgs([]string{"--no-clone", "svc/foo", "git@x:o/foo.git"})
	require.NoError(t, cmd.Execute())

	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	s := string(gi)
	assert.Contains(t, s, ".gogo.local\n")
	assert.Contains(t, s, ".gogo.local.yaml")
	assert.Contains(t, s, ".gogo.local.yml")
	assert.Contains(t, s, "svc/foo")
}
