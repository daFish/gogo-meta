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

func TestValidateWorkingCopyMissingDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"libs/api":"git@example.com:org/api.git"}}`), 0o644))

	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = oldW, oldE }()

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(wd) }()

	err := runValidate(nil, nil)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "directory missing")
	assert.Contains(t, buf.String(), "gogo migrate")
}

func TestValidateWorkingCopyAllPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"libs/api":"git@example.com:org/api.git"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "libs", "api"), 0o755))

	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = oldW, oldE }()

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(wd) }()

	err := runValidate(nil, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All 1 project directories present")
}
