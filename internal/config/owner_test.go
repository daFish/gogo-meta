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
