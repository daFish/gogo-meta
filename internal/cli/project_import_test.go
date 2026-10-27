package cli

import (
	"os"
	"path/filepath"
	"strings"
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

func TestProjectImportKeepsSchemaReference(t *testing.T) {
	const schemaURL = "https://raw.githubusercontent.com/daFish/gogo-meta/v1.0.0/internal/config/gogo.schema.json"
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{},"$schema":"`+schemaURL+`"}`), 0o644))
	config.SetOverlayFiles(nil)
	_ = captureOutput(t)
	initTestChdir(t, dir)

	cmd := newProjectImportCmd()
	cmd.SetArgs([]string{"--no-clone", "svc/foo", "git@x:o/foo.git"})
	require.NoError(t, cmd.Execute())

	written, err := os.ReadFile(filepath.Join(dir, ".gogo"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(written), "{\n  \"$schema\": \""+schemaURL+"\",\n"),
		"$schema must survive the rewrite as the first key:\n%s", written)
	assert.Contains(t, string(written), `"svc/foo": "git@x:o/foo.git"`)
}
