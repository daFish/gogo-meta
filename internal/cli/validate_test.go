package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWorkingCopySurfacesMergeError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	config.SetOverlayFiles([]string{"does-not-exist.yaml"})
	defer config.SetOverlayFiles(nil)
	buf := captureOutput(t)

	hasErrors := validateWorkingCopy(dir, nil, nil)
	assert.True(t, hasErrors, "broken merged config must be surfaced, not silently passed")
	assert.Contains(t, buf.String(), "Overlay config file not found")
}

func TestValidateWorkingCopyWarnsOnMismatchedLocal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local.yaml"),
		[]byte("projects:\n  b: urlB\n"), 0o644))
	config.SetOverlayFiles(nil)
	buf := captureOutput(t)

	hasErrors := validateWorkingCopy(dir, nil, nil)
	assert.False(t, hasErrors)
	assert.Contains(t, buf.String(), "will not be merged",
		"validate must surface the format-mismatch warning")
}

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

func TestValidateReportsUnknownGroupProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"libs/api":"git@example.com:org/api.git"},"groups":{"foo":["libs/nope"]}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "libs", "api"), 0o755))

	var buf bytes.Buffer
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = &buf, &buf
	defer func() { output.Writer, output.ErrWriter = oldW, oldE }()

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(wd) }()

	err := runValidate(nil, nil)
	require.Error(t, err)
	assert.Contains(t, buf.String(), `group "foo": unknown project "libs/nope"`)
}

// inValidatedDir writes files into a temp dir, chdirs into it and captures the
// console, so a test can call runValidate and read back what it printed.
func inValidatedDir(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}

	buf := &bytes.Buffer{}
	oldW, oldE := output.Writer, output.ErrWriter
	output.Writer, output.ErrWriter = buf, buf
	t.Cleanup(func() { output.Writer, output.ErrWriter = oldW, oldE })

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return buf
}

func TestValidateResolvesReferencesAgainstUnloadedOverlays(t *testing.T) {
	t.Run("group may name a project declared only in an overlay", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{
			".gogo":        `{"projects":{"api":"urlA"},"groups":{"deploy":["infra/tf"]}}`,
			".gogo.devops": `{"projects":{"infra/tf":"urlT"}}`,
		})
		require.NoError(t, os.MkdirAll("api", 0o755))

		require.NoError(t, runValidate(nil, nil))
		assert.NotContains(t, buf.String(), "unknown project")
	})

	t.Run("command may name a group declared only in an overlay", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{
			".gogo":        `{"projects":{"api":"urlA"},"commands":{"deploy":{"cmd":"make deploy","groups":["infra"]}}}`,
			".gogo.devops": `{"projects":{},"groups":{"infra":["api"]}}`,
		})
		require.NoError(t, os.MkdirAll("api", 0o755))

		require.NoError(t, runValidate(nil, nil))
		assert.NotContains(t, buf.String(), "unknown group")
	})

	t.Run("a reference no config file satisfies is still reported", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{
			".gogo":        `{"projects":{"api":"urlA"},"groups":{"deploy":["infra/tf"]}}`,
			".gogo.devops": `{"projects":{"something/else":"urlE"}}`,
		})
		require.NoError(t, os.MkdirAll("api", 0o755))

		require.Error(t, runValidate(nil, nil))
		assert.Contains(t, buf.String(), `group "deploy": unknown project "infra/tf"`)
	})

	t.Run("overlays next to a primary config in a parent directory count too", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{
			".gogo":        `{"projects":{"api":"urlA"},"groups":{"deploy":["infra/tf"]}}`,
			".gogo.devops": `{"projects":{"infra/tf":"urlT"}}`,
		})
		require.NoError(t, os.MkdirAll(filepath.Join("infra", "tf"), 0o755))
		require.NoError(t, os.MkdirAll("api", 0o755))

		// The cwd scan only ever sees .gogo.local; the primary config and the
		// overlay declaring infra/tf are one level up.
		require.NoError(t, os.MkdirAll("sub", 0o755))
		require.NoError(t, os.WriteFile(filepath.Join("sub", ".gogo.local"), []byte(`{"projects":{}}`), 0o644))
		require.NoError(t, os.Chdir("sub"))

		require.NoError(t, runValidate(nil, nil))
		assert.NotContains(t, buf.String(), "unknown project")
	})

	t.Run("an overlay that fails its own checks does not vouch for a project", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{
			".gogo":        `{"projects":{"api":"urlA"},"groups":{"deploy":["infra/tf"]}}`,
			".gogo.devops": `{"projects":{"infra/tf":"urlT"},"groups":{"empty":[]}}`,
		})
		require.NoError(t, os.MkdirAll("api", 0o755))

		require.Error(t, runValidate(nil, nil))
		assert.Contains(t, buf.String(), `group "deploy": unknown project "infra/tf"`)
	})
}

func TestValidateReportsAFileProblemOnce(t *testing.T) {
	t.Run("a malformed config in the cwd is printed once", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{".gogo": `{"projects":{"api":"urlA",`})

		require.Error(t, runValidate(nil, nil))
		assert.Equal(t, 1, strings.Count(buf.String(), "unexpected end of JSON input"),
			"the per-file loop already reported it")
	})

	t.Run("an overlay outside the per-file loop is still reported", func(t *testing.T) {
		buf := inValidatedDir(t, map[string]string{".gogo": `{"projects":{"api":"urlA"}}`})
		require.NoError(t, os.MkdirAll("api", 0o755))
		config.SetOverlayFiles([]string{"nope.yaml"})
		t.Cleanup(func() { config.SetOverlayFiles(nil) })

		require.Error(t, runValidate(nil, nil))
		assert.Contains(t, buf.String(), "Overlay config file not found")
	})
}
