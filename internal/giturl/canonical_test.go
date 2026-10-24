package giturl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalEqual(t *testing.T) {
	sameRepository := []struct {
		name       string
		configured string
		onDisk     string
	}{
		{"https vs scp", "https://github.com/org/api.git", "git@github.com:org/api.git"},
		{"missing .git suffix", "https://github.com/org/api", "https://github.com/org/api.git"},
		{"trailing slash", "https://github.com/org/api.git", "https://github.com/org/api.git/"},
		{"ssh vs scp", "ssh://git@github.com/org/api.git", "git@github.com:org/api.git"},
		{"path case on a case-insensitive forge", "https://github.com/Org/Api.git", "https://github.com/org/api.git"},
		{"host case", "https://GitHub.com/org/api.git", "https://github.com/org/api.git"},
		{"ssh port vs https", "ssh://git@git.example.com:2222/org/api.git", "https://git.example.com/org/api.git"},
		{"user info", "https://user@git.example.com/org/api.git", "https://git.example.com/org/api"},
		{"duplicate slashes", "https://github.com/org//api.git", "https://github.com/org/api.git"},
		{"scp with IPv6 host", "git@[::1]:org/api.git", "ssh://git@[::1]/org/api.git"},
		{"local trailing slash", "/remotes/api.git/", "/remotes/api.git"},
		{"file scheme vs local path", "file:///remotes/api.git", "/remotes/api.git"},
		{"surrounding whitespace", " git@github.com:org/api.git\n", "git@github.com:org/api.git"},
	}
	for _, tc := range sameRepository {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, Canonical(tc.configured), Canonical(tc.onDisk))
		})
	}
}

func TestCanonicalDistinct(t *testing.T) {
	differentRepositories := []struct {
		name string
		a, b string
	}{
		{"different repository", "git@github.com:org/api.git", "git@github.com:org/web.git"},
		{"different owner", "https://github.com/org/api.git", "https://github.com/fork/api.git"},
		{"different host", "https://github.com/org/api.git", "https://gitlab.com/org/api.git"},
		{"path case on an unknown host", "https://git.example.com/Org/api.git", "https://git.example.com/org/api.git"},
		{"local path case", "/remotes/Api.git", "/remotes/api.git"},
		{"local path vs network remote", "/github.com/org/api.git", "https://github.com/org/api.git"},
		{"Windows drive vs scp host", "C:/repos/api.git", "cc:/repos/api.git"},
		{"empty vs anything", "", "git@github.com:org/api.git"},
	}
	for _, tc := range differentRepositories {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, Canonical(tc.a), Canonical(tc.b))
		})
	}
}

func TestCanonicalEmpty(t *testing.T) {
	assert.Empty(t, Canonical(""))
	assert.Empty(t, Canonical("  "))
}
