package ssh

import (
	"context"
	"testing"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/stretchr/testify/assert"
)

type stubExecutor struct {
	exitCode int
	gotCmd   string
}

func (s *stubExecutor) Execute(_ context.Context, command string, _ executor.Options) (*executor.Result, error) {
	s.gotCmd = command
	return &executor.Result{ExitCode: s.exitCode}, nil
}

func TestAddHostKeyUsesExecutor(t *testing.T) {
	ok := &stubExecutor{exitCode: 0}
	assert.True(t, AddHostKey(ok, "example.com"))
	assert.Contains(t, ok.gotCmd, "ssh-keyscan")
	assert.Contains(t, ok.gotCmd, "example.com")

	bad := &stubExecutor{exitCode: 1}
	assert.False(t, AddHostKey(bad, "example.com"))
}

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
		urls := []string{"https://github.com/org/repo.git"}
		hosts := ExtractUniqueSSHHosts(urls)
		assert.Nil(t, hosts)
	})

	t.Run("handles empty input", func(t *testing.T) {
		hosts := ExtractUniqueSSHHosts([]string{})
		assert.Nil(t, hosts)
	})
}
