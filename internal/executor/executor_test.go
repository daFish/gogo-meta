package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellExecutor_Execute(t *testing.T) {
	exec := NewShellExecutor()
	ctx := context.Background()

	t.Run("simple command stdout", func(t *testing.T) {
		result, err := exec.Execute(ctx, "echo hello", Options{Cwd: t.TempDir()})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Equal(t, "hello", result.Stdout)
		assert.False(t, result.TimedOut)
	})

	t.Run("captures stderr", func(t *testing.T) {
		result, err := exec.Execute(ctx, "echo error >&2", Options{Cwd: t.TempDir()})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Equal(t, "error", result.Stderr)
	})

	t.Run("nonzero exit code", func(t *testing.T) {
		result, err := exec.Execute(ctx, "exit 42", Options{Cwd: t.TempDir()})
		require.NoError(t, err)
		assert.Equal(t, 42, result.ExitCode)
	})

	t.Run("multiline output", func(t *testing.T) {
		result, err := exec.Execute(ctx, "echo 'line1\nline2'", Options{Cwd: t.TempDir()})
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "line1")
		assert.Contains(t, result.Stdout, "line2")
	})

	t.Run("respects cwd", func(t *testing.T) {
		dir := t.TempDir()
		result, err := exec.Execute(ctx, "pwd", Options{Cwd: dir})
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, dir)
	})

	t.Run("passes environment variables", func(t *testing.T) {
		result, err := exec.Execute(ctx, "echo $TEST_VAR", Options{
			Cwd: t.TempDir(),
			Env: []string{"TEST_VAR=hello_world"},
		})
		require.NoError(t, err)
		assert.Equal(t, "hello_world", result.Stdout)
	})

	t.Run("timeout returns exit code 124", func(t *testing.T) {
		result, err := exec.Execute(ctx, "sleep 10", Options{
			Cwd:     t.TempDir(),
			Timeout: 100 * time.Millisecond,
		})
		require.NoError(t, err)
		assert.True(t, result.TimedOut)
		assert.Equal(t, 124, result.ExitCode)
	})
}
