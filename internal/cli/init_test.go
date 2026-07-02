package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/daFish/gogo-meta/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestChdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// silenceOutput redirects output.Writer/ErrWriter to a buffer for the test.
func silenceOutput(t *testing.T) {
	t.Helper()
	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	t.Cleanup(func() { output.Writer, output.ErrWriter = oldW, oldE })
}

func TestInitDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	initTestChdir(t, dir)
	silenceOutput(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"init"})
	require.NotPanics(t, func() { _ = root.Execute() })
	assert.FileExists(t, filepath.Join(dir, ".gogo"))
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	initTestChdir(t, dir)
	silenceOutput(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"keep":"git@x:o/r.git"}}`), 0o644))

	root := NewRootCommand("test")
	root.SetArgs([]string{"init", "--force"})
	require.NoError(t, root.Execute())

	b, err := os.ReadFile(filepath.Join(dir, ".gogo"))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "keep") // overwritten to default empty config
}

func TestInitPrintsLocalHint(t *testing.T) {
	dir := t.TempDir()
	initTestChdir(t, dir)
	buf := captureOutput(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"init"})
	require.NoError(t, root.Execute())

	assert.Contains(t, buf.String(),
		"Tip: Create a .gogo.local file for personal overrides")
}

func TestInitAddsLocalConfigToGitignore(t *testing.T) {
	dir := t.TempDir()
	initTestChdir(t, dir)
	silenceOutput(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"init"})
	require.NoError(t, root.Execute())

	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, ".gogo.local\n")
	assert.Contains(t, s, ".gogo.local.yaml")
	assert.Contains(t, s, ".gogo.local.yml")
}
