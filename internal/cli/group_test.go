package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
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

// TestGroupFlagPositions drives the real command tree the way the binary does,
// because --group exists twice: as a persistent flag on the root and as a local
// flag on every filter command. cobra merges the two into the single local flag,
// so `gogo --group foo exec ...` and `gogo exec --group foo ...` must end up in
// the same place — and getStringFlag's inherited-flag fallback must not be what
// this depends on.
func TestGroupFlagPositions(t *testing.T) {
	setupGroupRepo(t)

	resolveVia := func(t *testing.T, args ...string) filter.Options {
		t.Helper()
		root := NewRootCommand("test")
		execCmd, _, err := root.Find([]string{"exec"})
		require.NoError(t, err)

		var got filter.Options
		execCmd.RunE = func(cmd *cobra.Command, _ []string) error {
			var err error
			got, err = resolveFilterOptions(cmd)
			return err
		}
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		require.NoError(t, root.Execute())
		return got
	}

	t.Run("before the subcommand", func(t *testing.T) {
		opts := resolveVia(t, "--group", "foo", "exec", "true")
		assert.Equal(t, []string{"a", "b"}, opts.GroupOnly)
	})

	t.Run("after the subcommand", func(t *testing.T) {
		opts := resolveVia(t, "exec", "--group", "bar", "true")
		assert.Equal(t, []string{"c"}, opts.GroupOnly)
	})

	t.Run("comma-separated union in root position", func(t *testing.T) {
		opts := resolveVia(t, "--group", "foo,bar", "exec", "true")
		assert.Equal(t, []string{"a", "b", "c"}, opts.GroupOnly)
	})

	t.Run("no group flag leaves GroupOnly empty", func(t *testing.T) {
		opts := resolveVia(t, "exec", "true")
		assert.Empty(t, opts.GroupOnly)
	})

	t.Run("an unknown group fails the command", func(t *testing.T) {
		root := NewRootCommand("test")
		root.SetArgs([]string{"--group", "nope", "exec", "true"})
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)

		err := root.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown group "nope"`)
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

// TestGroupSpanningLocalOverlay covers the seam between named groups and the
// personal .gogo.local overlay: a shared group may name a project that only the
// overlay declares. Filtering resolves it because the merged config is the
// effective one, and validate does not report it as unknown because it reads
// every .gogo* file beside the primary, the overlay included.
func TestGroupSpanningLocalOverlay(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte(`{
	  "projects": {"shared": "urlShared"},
	  "groups": {"mixed": ["shared", "personal"]}
	}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"), []byte(`{
	  "projects": {"personal": "urlPersonal"}
	}`), 0o644))
	for _, p := range []string{"shared", "personal"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, p), 0o755))
	}
	config.SetOverlayFiles(nil)
	buf := captureOutput(t)
	initTestChdir(t, dir)

	opts, err := resolveFilterOptions(filterCmd(t, map[string]string{"group": "mixed"}))
	require.NoError(t, err)
	assert.Equal(t, []string{"personal", "shared"}, opts.GroupOnly,
		"a group may span the shared config and the local overlay")

	require.NoError(t, runValidate(nil, nil),
		"an overlay-declared group member is not an unknown project")
	assert.NotContains(t, buf.String(), "unknown project")
}

// TestGroupFlagAnnouncesOverlayOnce guards the seam where --group made a loop
// command read the merged config twice: the plumbing read behind the flag must
// stay quiet, so the overlay banner appears once per command.
func TestGroupFlagAnnouncesOverlayOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo"), []byte(`{
	  "projects": {"a": "urlA"},
	  "groups": {"foo": ["a"]}
	}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gogo.local"), []byte(`{
	  "projects": {"b": "urlB"}
	}`), 0o644))
	for _, p := range []string{"a", "b"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, p), 0o755))
	}
	config.SetOverlayFiles(nil)
	buf := captureOutput(t)
	initTestChdir(t, dir)

	cmd := newExecCmd()
	cmd.SetArgs([]string{"--group", "foo", "true"})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, 1, strings.Count(buf.String(), "Using local overlay config: .gogo.local"),
		"the overlay banner belongs to the command's own config load, not the filter plumbing")
}
