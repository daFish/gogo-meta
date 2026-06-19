package ssh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubExecutor struct {
	exitCode int
	gotCmd   string
	stdout   string
}

func (s *stubExecutor) Execute(_ context.Context, command string, _ executor.Options) (*executor.Result, error) {
	s.gotCmd = command
	return &executor.Result{ExitCode: s.exitCode, Stdout: s.stdout}, nil
}

func (s *stubExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts executor.Options) (*executor.Result, error) {
	return s.Execute(ctx, strings.TrimSpace(name+" "+strings.Join(args, " ")), opts)
}

func TestAddHostKeyUsesExecutor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))

	ok := &stubExecutor{exitCode: 0, stdout: "example.com ssh-rsa AAAAKEY"}
	assert.True(t, AddHostKey(context.Background(), ok, "example.com"))
	assert.Contains(t, ok.gotCmd, "ssh-keyscan")
	assert.Contains(t, ok.gotCmd, "example.com")

	bad := &stubExecutor{exitCode: 1}
	assert.False(t, AddHostKey(context.Background(), bad, "example.com"))
}

func TestAddHostKeyAppendsScannedKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))

	stub := &stubExecutor{exitCode: 0, stdout: "example.com ssh-rsa AAAAKEY"}
	ok := AddHostKey(context.Background(), stub, "example.com")
	require.True(t, ok)

	content, err := os.ReadFile(filepath.Join(home, ".ssh", "known_hosts"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "example.com ssh-rsa AAAAKEY")
}

func TestAddHostKeyFailsOnNonZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stub := &stubExecutor{exitCode: 1}
	assert.False(t, AddHostKey(context.Background(), stub, "example.com"))
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
