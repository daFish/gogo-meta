package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkAllProjects_MaliciousDepNameCannotEscapeNodeModules(t *testing.T) {
	_ = captureOutput(t)
	root := t.TempDir()

	consumerDir := filepath.Join(root, "consumer")
	providerDir := filepath.Join(root, "provider")
	require.NoError(t, os.MkdirAll(consumerDir, 0o755))
	require.NoError(t, os.MkdirAll(providerDir, 0o755))

	victimDir := filepath.Join(root, "victim")
	require.NoError(t, os.MkdirAll(victimDir, 0o755))
	sentinel := filepath.Join(victimDir, "keep.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("important"), 0o644))

	// From consumer/node_modules, "../../victim" resolves to root/victim.
	malicious := filepath.ToSlash(filepath.Join("..", "..", "victim"))

	pkgs := map[string]projectInfo{
		malicious: {
			path:    providerDir,
			pkgJSON: packageJSON{Name: malicious},
		},
		"consumer": {
			path: consumerDir,
			pkgJSON: packageJSON{
				Name:         "consumer",
				Dependencies: map[string]string{malicious: "1.0.0"},
			},
		},
	}

	count := linkAllProjects(pkgs)

	assert.FileExists(t, sentinel, "traversal dep name must not delete files outside node_modules")

	info, err := os.Lstat(victimDir)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&os.ModeSymlink, "victim dir must not be replaced by a symlink")

	assert.Equal(t, 0, count, "a traversal dependency name must not be linked")
}

func assertSymlinkTo(t *testing.T, link, want string) {
	t.Helper()
	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink, "%s should be a symlink", link)
	dest, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, want, dest)
}

func TestLinkAllProjects_LinksPlainAndScopedDependencies(t *testing.T) {
	_ = captureOutput(t)
	root := t.TempDir()

	consumerDir := filepath.Join(root, "consumer")
	libDir := filepath.Join(root, "lib")
	uiDir := filepath.Join(root, "ui")
	for _, d := range []string{consumerDir, libDir, uiDir} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}

	pkgs := map[string]projectInfo{
		"lib":       {path: libDir, pkgJSON: packageJSON{Name: "lib"}},
		"@scope/ui": {path: uiDir, pkgJSON: packageJSON{Name: "@scope/ui"}},
		"consumer": {
			path: consumerDir,
			pkgJSON: packageJSON{
				Name:            "consumer",
				Dependencies:    map[string]string{"lib": "1.0.0"},
				DevDependencies: map[string]string{"@scope/ui": "1.0.0"},
			},
		},
	}

	count := linkAllProjects(pkgs)

	assert.Equal(t, 2, count)
	assertSymlinkTo(t, filepath.Join(consumerDir, "node_modules", "lib"), libDir)
	assertSymlinkTo(t, filepath.Join(consumerDir, "node_modules", "@scope", "ui"), uiDir)
}

func TestSafeLinkTarget(t *testing.T) {
	consumer := filepath.FromSlash("/repo/consumer")
	base := filepath.Join(consumer, "node_modules")

	tests := []struct {
		name    string
		depName string
		want    string
		wantErr bool
	}{
		{"plain name", "lib", filepath.Join(base, "lib"), false},
		{"scoped name", "@scope/ui", filepath.Join(base, "@scope", "ui"), false},
		{"parent traversal", filepath.ToSlash(filepath.Join("..", "..", "victim")), "", true},
		{"bare dotdot", "..", "", true},
		{"embedded traversal", "a/../../b", "", true},
		{"absolute name", filepath.FromSlash("/etc/passwd"), "", true},
		{"empty name", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeLinkTarget(consumer, tt.depName)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLinkAllProjects_SymlinkedNodeModulesCannotEscape(t *testing.T) {
	_ = captureOutput(t)
	root := t.TempDir()

	consumerDir := filepath.Join(root, "consumer")
	providerDir := filepath.Join(root, "provider")
	require.NoError(t, os.MkdirAll(consumerDir, 0o755))
	require.NoError(t, os.MkdirAll(providerDir, 0o755))

	victimDir := filepath.Join(root, "victim")
	require.NoError(t, os.MkdirAll(victimDir, 0o755))
	sentinel := filepath.Join(victimDir, "keep.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("important"), 0o644))

	// Malicious repo ships node_modules as a symlink pointing outside the tree.
	require.NoError(t, os.Symlink(victimDir, filepath.Join(consumerDir, "node_modules")))

	pkgs := map[string]projectInfo{
		"lib": {path: providerDir, pkgJSON: packageJSON{Name: "lib"}},
		"consumer": {
			path: consumerDir,
			pkgJSON: packageJSON{
				Name:         "consumer",
				Dependencies: map[string]string{"lib": "1.0.0"},
			},
		},
	}

	count := linkAllProjects(pkgs)

	_, err := os.Lstat(filepath.Join(victimDir, "lib"))
	assert.True(t, os.IsNotExist(err), "no entry may be created through a symlinked node_modules")
	assert.FileExists(t, sentinel)
	assert.Equal(t, 0, count, "link through a symlinked node_modules must be refused")
}

func TestLinkAllProjects_ReplacesStaleLinkWhenNodeModulesIsReal(t *testing.T) {
	_ = captureOutput(t)
	root := t.TempDir()

	consumerDir := filepath.Join(root, "consumer")
	providerDir := filepath.Join(root, "provider")
	staleDir := filepath.Join(root, "stale")
	for _, d := range []string{consumerDir, providerDir, staleDir} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}

	// Steady state from a previous run: a real node_modules holding a stale link.
	nm := filepath.Join(consumerDir, "node_modules")
	require.NoError(t, os.MkdirAll(nm, 0o755))
	require.NoError(t, os.Symlink(staleDir, filepath.Join(nm, "lib")))

	pkgs := map[string]projectInfo{
		"lib": {path: providerDir, pkgJSON: packageJSON{Name: "lib"}},
		"consumer": {
			path: consumerDir,
			pkgJSON: packageJSON{
				Name:         "consumer",
				Dependencies: map[string]string{"lib": "1.0.0"},
			},
		},
	}

	count := linkAllProjects(pkgs)

	assert.Equal(t, 1, count)
	assertSymlinkTo(t, filepath.Join(nm, "lib"), providerDir)
}
