package giturl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	allowed := []string{
		"https://github.com/org/repo.git",
		"http://example.com/org/repo.git",
		"git@github.com:org/repo.git",
		"ssh://git@github.com:2222/org/repo.git",
		"git://example.com/org/repo.git",
		"file:///srv/git/repo.git",
		"ssh://git@[::1]/org/repo.git",
	}
	for _, u := range allowed {
		t.Run("allow/"+u, func(t *testing.T) {
			require.NoError(t, Validate(u))
		})
	}

	rejected := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"leading dash", "--upload-pack=touch /tmp/x"},
		{"ext transport", "ext::sh -c 'touch /tmp/x'"},
		{"fd transport", "fd::17"},
	}
	for _, tc := range rejected {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			assert.Error(t, Validate(tc.url))
		})
	}
}
