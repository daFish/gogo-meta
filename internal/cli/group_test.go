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
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGroupRepo creates a meta repo with four projects and two groups, and
// chdirs into it for the duration of the test.
func setupGroupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte(`{
	  "projects": {"a": "urlA", "b": "urlB", "c": "urlC", "d": "urlD"},
	  "groups": {"foo": ["a", "b"], "bar": ["c"]}
	}`), 0o644))
	for _, p := range []string{"a", "b", "c", "d"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, p), 0o755))
	}
	config.SetOverlayFiles(nil)

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return dir
}

// filterCmd builds a command carrying the shared filter flags with the given
// flags already set.
func filterCmd(t *testing.T, flags map[string]string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	for name, value := range flags {
		require.NoError(t, cmd.Flags().Set(name, value))
	}
	return cmd
}

func TestResolveFilterOptionsGroupFlag(t *testing.T) {
	setupGroupRepo(t)

	t.Run("resolves a group to its project paths", func(t *testing.T) {
		opts, err := resolveFilterOptions(filterCmd(t, map[string]string{"group": "foo"}))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, opts.GroupOnly)
	})

	t.Run("unions multiple groups", func(t *testing.T) {
		opts, err := resolveFilterOptions(filterCmd(t, map[string]string{"group": "foo,bar"}))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, opts.GroupOnly)
	})

	t.Run("leaves GroupOnly empty without the flag", func(t *testing.T) {
		opts, err := resolveFilterOptions(filterCmd(t, nil))
		require.NoError(t, err)
		assert.Empty(t, opts.GroupOnly)
	})

	t.Run("errors on an unknown group", func(t *testing.T) {
		_, err := resolveFilterOptions(filterCmd(t, map[string]string{"group": "nope"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown group "nope"`)
		assert.Contains(t, err.Error(), "Available groups: bar, foo")
	})
}

func TestRunLoopCommandWithGroup(t *testing.T) {
	setupGroupRepo(t)
	_ = captureOutput(t)

	opts, err := resolveLoopOptions(filterCmd(t, map[string]string{"group": "foo"}))
	require.NoError(t, err)

	var mu sync.Mutex
	var ran []string
	command := loop.CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		mu.Lock()
		ran = append(ran, projectPath)
		mu.Unlock()
		return &executor.Result{ExitCode: 0}, nil
	})

	require.NoError(t, runLoopCommand(context.Background(), command, opts))
	assert.Equal(t, []string{"a", "b"}, ran)
}

func TestRunLoopCommandGroupIntersectsWithIncludeOnly(t *testing.T) {
	setupGroupRepo(t)
	_ = captureOutput(t)

	opts, err := resolveLoopOptions(filterCmd(t, map[string]string{"group": "foo", "include-only": "b,c"}))
	require.NoError(t, err)

	var ran []string
	command := loop.CommandFn(func(_ context.Context, _, projectPath string) (*executor.Result, error) {
		ran = append(ran, projectPath)
		return &executor.Result{ExitCode: 0}, nil
	})

	require.NoError(t, runLoopCommand(context.Background(), command, opts))
	assert.Equal(t, []string{"b"}, ran)
}

func TestRunGroupFilterFromCommandConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte(`{
	  "projects": {"a": "urlA", "b": "urlB", "c": "urlC"},
	  "groups": {"foo": ["a", "b"]},
	  "commands": {"hello": {"cmd": "echo hallo", "groups": ["foo"]}}
	}`), 0o644))
	for _, p := range []string{"a", "b", "c"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, p), 0o755))
	}
	config.SetOverlayFiles(nil)

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	buf := captureOutput(t)
	cmd := filterCmd(t, nil)
	cmd.Flags().BoolP("list", "l", false, "")
	cmd.SetContext(context.Background())
	require.NoError(t, runRun(cmd, []string{"hello"}))

	// Each project the command ran in gets a header line of its own.
	out := buf.String()
	assert.Contains(t, out, output.ArrowSymbol+" a\n")
	assert.Contains(t, out, output.ArrowSymbol+" b\n")
	assert.NotContains(t, out, output.ArrowSymbol+" c\n")
	assert.Contains(t, out, "hallo")
}
