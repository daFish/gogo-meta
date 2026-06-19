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
