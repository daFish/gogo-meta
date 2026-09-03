package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSSHHost(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		expect string
	}{
		{"git@host:path format", "git@github.com:user/repo.git", "github.com"},
		{"ssh:// format", "ssh://git@github.com/user/repo.git", "github.com"},
		{"ssh:// with port", "ssh://git@github.com:2222/user/repo.git", "github.com"},
		{"custom git host", "git@gitlab.example.com:user/repo.git", "gitlab.example.com"},
		{"https URL returns empty", "https://github.com/user/repo.git", ""},
		{"http URL returns empty", "http://github.com/user/repo.git", ""},
		{"file URL returns empty", "file:///path/to/repo", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, ExtractSSHHost(tt.url))
		})
	}
}

func TestExtractUniqueSSHHosts(t *testing.T) {
	t.Run("extracts unique hosts", func(t *testing.T) {
		urls := []string{
			"git@github.com:org/repo1.git",
			"git@github.com:org/repo2.git",
			"git@gitlab.com:org/repo3.git",
			"https://github.com/org/repo4.git",
		}
		hosts := ExtractUniqueSSHHosts(urls)
		assert.Equal(t, []string{"github.com", "gitlab.com"}, hosts)
	})

	t.Run("returns nil for no SSH URLs", func(t *testing.T) {
		hosts := ExtractUniqueSSHHosts([]string{"https://github.com/org/repo.git"})
		assert.Nil(t, hosts)
	})

	t.Run("handles empty input", func(t *testing.T) {
		hosts := ExtractUniqueSSHHosts([]string{})
		assert.Nil(t, hosts)
	})
}

func TestUnverifiedSSHHostsReportsUnknownWithoutWriting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))

	hosts := UnverifiedSSHHosts([]string{"git@github.com:o/r.git"})
	assert.Equal(t, []string{"github.com"}, hosts)

	_, err := os.Stat(filepath.Join(home, ".ssh", "known_hosts"))
	assert.True(t, os.IsNotExist(err), "gogo must not add host keys automatically")
}

func TestUnverifiedSSHHostsSkipsKnownAndNonSSH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"),
		[]byte("gitlab.com ssh-rsa AAAAKNOWN\n"), 0o600))

	hosts := UnverifiedSSHHosts([]string{
		"git@gitlab.com:o/r.git",
		"git@github.com:o/r.git",
		"https://example.com/o/r.git",
	})
	assert.Equal(t, []string{"github.com"}, hosts)
}
