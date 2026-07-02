package cli

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunLoopCommandRunsAcrossProjects(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA","b":"urlB"}}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b"), 0o755))
	config.SetOverlayFiles(nil)
	_ = captureOutput(t)

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	var mu sync.Mutex
	ran := map[string]bool{}
	command := loop.CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		mu.Lock()
		ran[projectPath] = true
		mu.Unlock()
		return &executor.Result{ExitCode: 0}, nil
	})

	err := runLoopCommand(context.Background(), command, loop.Options{})
	require.NoError(t, err)
	assert.True(t, ran["a"])
	assert.True(t, ran["b"])
}

func TestResolveConfigPrintsLocalOverlayInfo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"),
		[]byte(`{"projects":{"b":"urlB"}}`), 0o644))
	config.SetOverlayFiles(nil)
	buf := captureOutput(t)
	initTestChdir(t, dir)

	result, err := resolveConfig()
	require.NoError(t, err)
	assert.Equal(t, "urlB", result.Config.Projects["b"])
	assert.Contains(t, buf.String(), "Using local overlay config: .gogo.local")
}

func TestResolveConfigWarnsOnMismatchedLocal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local.yaml"),
		[]byte("projects:\n  b: urlB\n"), 0o644))
	config.SetOverlayFiles(nil)
	buf := captureOutput(t)
	initTestChdir(t, dir)

	_, err := resolveConfig()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "will not be merged")
}

func TestResolveConfigNoOverlayNoInfo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"),
		[]byte(`{"projects":{"a":"urlA"}}`), 0o644))
	config.SetOverlayFiles(nil)
	buf := captureOutput(t)
	initTestChdir(t, dir)

	_, err := resolveConfig()
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "overlay config")
}
