package giturl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"user and password", "https://deploy:ghp_SECRET@github.com/org/api.git", "https://github.com/org/api.git"},
		{"token as user name", "https://ghp_SECRET@github.com/org/api.git", "https://github.com/org/api.git"},
		{"at sign in the password", "https://deploy:p@ss@github.com/org/api.git", "https://github.com/org/api.git"},
		{"ssh user", "ssh://git@git.example.com:2222/org/api.git", "ssh://git.example.com:2222/org/api.git"},
		{"unparseable URL", "https://deploy:ghp_SECRET@github.com/org/%zz", "https://github.com/org/%zz"},
		{"no path", "https://ghp_SECRET@github.com", "https://github.com"},
		{"at sign in the path only", "https://github.com/org/api@v1.git", "https://github.com/org/api@v1.git"},
		{"at sign in the query only", "https://github.com?ref=a@b", "https://github.com?ref=a@b"},
		{"no user info", "https://github.com/org/api.git", "https://github.com/org/api.git"},
		{"scp-style", "git@github.com:org/api.git", "git@github.com:org/api.git"},
		{"local path", "/remotes/api.git", "/remotes/api.git"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Redact(tc.in))
		})
	}
}
