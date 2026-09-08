package gitstate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedExecutor answers each git invocation from a table keyed by the
// subcommand, and records the argv it was asked to run.
type scriptedExecutor struct {
	results map[string]*executor.Result
	err     error
	calls   [][]string
}

func (s *scriptedExecutor) Execute(_ context.Context, _ string, _ executor.Options) (*executor.Result, error) {
	return &executor.Result{}, nil
}

func (s *scriptedExecutor) ExecuteArgs(_ context.Context, _ string, args []string, _ executor.Options) (*executor.Result, error) {
	s.calls = append(s.calls, args)
	if s.err != nil {
		return nil, s.err
	}
	if result, ok := s.results[args[0]]; ok {
		return result, nil
	}
	return &executor.Result{ExitCode: 0}, nil
}

func TestParsePorcelainV2(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want Status
	}{
		{
			name: "clean branch with upstream",
			out: "# branch.oid 4f2a9c\n" +
				"# branch.head main\n" +
				"# branch.upstream origin/main\n" +
				"# branch.ab +0 -0\n",
			want: Status{Head: "4f2a9c", Branch: "main", Upstream: "origin/main"},
		},
		{
			name: "detached HEAD",
			out:  "# branch.oid 4f2a9c\n# branch.head (detached)\n",
			want: Status{Head: "4f2a9c", Detached: true},
		},
		{
			name: "no upstream",
			out:  "# branch.oid 4f2a9c\n# branch.head feature\n",
			want: Status{Head: "4f2a9c", Branch: "feature"},
		},
		{
			name: "unborn branch",
			out:  "# branch.oid (initial)\n# branch.head main\n",
			want: Status{Branch: "main"},
		},
		{
			name: "changed entry marks dirty",
			out: "# branch.oid 4f2a9c\n# branch.head main\n# branch.upstream origin/main\n" +
				"1 .M N... 100644 100644 100644 aaa bbb internal/cli/run.go\n",
			want: Status{Head: "4f2a9c", Branch: "main", Upstream: "origin/main", Dirty: true},
		},
		{
			name: "renamed entry marks dirty",
			out: "# branch.oid 4f2a9c\n# branch.head main\n" +
				"2 R. N... 100644 100644 100644 aaa bbb R100 new\told\n",
			want: Status{Head: "4f2a9c", Branch: "main", Dirty: true},
		},
		{
			name: "unmerged entry marks dirty",
			out: "# branch.oid 4f2a9c\n# branch.head main\n" +
				"u UU N... 100644 100644 100644 100644 aaa bbb ccc conflicted.go\n",
			want: Status{Head: "4f2a9c", Branch: "main", Dirty: true},
		},
		{
			name: "untracked entry marks dirty",
			out:  "# branch.oid 4f2a9c\n# branch.head main\n? notes.md\n",
			want: Status{Head: "4f2a9c", Branch: "main", Dirty: true},
		},
		{
			name: "empty output",
			out:  "",
			want: Status{},
		},
		{
			name: "carriage returns are tolerated",
			out:  "# branch.oid 4f2a9c\r\n# branch.head main\r\n# branch.upstream origin/main\r\n",
			want: Status{Head: "4f2a9c", Branch: "main", Upstream: "origin/main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParsePorcelainV2(tt.out))
		})
	}
}

func TestParsePorcelainV2IgnoresBranchAB(t *testing.T) {
	// branch.ab is computed before any fetch, so it is stale exactly when it
	// would matter. AheadBehind is the source of truth instead.
	status := ParsePorcelainV2("# branch.oid 4f2a9c\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +9 -9\n")
	assert.Equal(t, Status{Head: "4f2a9c", Branch: "main", Upstream: "origin/main"}, status)
}

func TestStatusUnborn(t *testing.T) {
	assert.True(t, ParsePorcelainV2("# branch.oid (initial)\n# branch.head main\n").Unborn())
	assert.False(t, ParsePorcelainV2("# branch.oid 4f2a9c\n# branch.head main\n").Unborn())
}

func TestInspectRunsReadOnlyPlumbing(t *testing.T) {
	ex := &scriptedExecutor{results: map[string]*executor.Result{
		"status": {ExitCode: 0, Stdout: "# branch.oid 4f2a9c\n# branch.head main\n# branch.upstream origin/main\n"},
	}}

	status, err := Inspect(context.Background(), ex, "/repo")
	require.NoError(t, err)
	assert.Equal(t, "main", status.Branch)
	assert.Equal(t, "origin/main", status.Upstream)
	require.Len(t, ex.calls, 1)
	assert.Equal(t, []string{"status", "--porcelain=v2", "--branch"}, ex.calls[0])
}

func TestInspectReportsFailure(t *testing.T) {
	t.Run("non-zero exit", func(t *testing.T) {
		ex := &scriptedExecutor{results: map[string]*executor.Result{
			"status": {ExitCode: 128, Stderr: "fatal: not a git repository\n"},
		}}
		_, err := Inspect(context.Background(), ex, "/repo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})

	t.Run("executor error", func(t *testing.T) {
		ex := &scriptedExecutor{err: errors.New("exec boom")}
		_, err := Inspect(context.Background(), ex, "/repo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exec boom")
	})
}

func TestAheadBehind(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		exitCode   int
		stderr     string
		wantAhead  int
		wantBehind int
		wantErr    string
	}{
		{name: "up to date", stdout: "0\t0\n"},
		{name: "ahead only", stdout: "3\t0\n", wantAhead: 3},
		{name: "behind only", stdout: "0\t5\n", wantBehind: 5},
		{name: "diverged", stdout: "2\t7\n", wantAhead: 2, wantBehind: 7},
		{name: "no upstream", exitCode: 128, stderr: "fatal: no upstream configured\n", wantErr: "no upstream configured"},
		{name: "unparsable", stdout: "who knows\n", wantErr: "unexpected rev-list output"},
		{name: "short output", stdout: "4\n", wantErr: "unexpected rev-list output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &scriptedExecutor{results: map[string]*executor.Result{
				"rev-list": {ExitCode: tt.exitCode, Stdout: tt.stdout, Stderr: tt.stderr},
			}}

			ahead, behind, err := AheadBehind(context.Background(), ex, "/repo")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAhead, ahead)
			assert.Equal(t, tt.wantBehind, behind)
			assert.Equal(t, []string{"rev-list", "--left-right", "--count", "HEAD...@{u}"}, ex.calls[0])
		})
	}
}

func TestFastForwardUsesFFOnly(t *testing.T) {
	ex := &scriptedExecutor{}
	_, err := FastForward(context.Background(), ex, "/repo")
	require.NoError(t, err)
	require.Len(t, ex.calls, 1)
	assert.Equal(t, []string{"merge", "--ff-only", "@{u}"}, ex.calls[0],
		"a merge commit must never be possible")
}

func TestFetchTouchesOnlyRemoteRefs(t *testing.T) {
	ex := &scriptedExecutor{}
	_, err := Fetch(context.Background(), ex, "/repo")
	require.NoError(t, err)
	require.Len(t, ex.calls, 1)
	assert.Equal(t, []string{"fetch"}, ex.calls[0])
}

func TestNoPlumbingRebasesOrStashes(t *testing.T) {
	ex := &scriptedExecutor{results: map[string]*executor.Result{
		"status":   {Stdout: "# branch.oid a\n# branch.head main\n# branch.upstream origin/main\n"},
		"rev-list": {Stdout: "0\t1\n"},
	}}

	ctx := context.Background()
	_, err := Inspect(ctx, ex, "/repo")
	require.NoError(t, err)
	_, err = Fetch(ctx, ex, "/repo")
	require.NoError(t, err)
	_, _, err = AheadBehind(ctx, ex, "/repo")
	require.NoError(t, err)
	_, err = FastForward(ctx, ex, "/repo")
	require.NoError(t, err)

	for _, call := range ex.calls {
		joined := strings.Join(call, " ")
		for _, forbidden := range []string{"rebase", "stash", "checkout", "reset", "pull"} {
			assert.NotContains(t, joined, forbidden, "gitstate must never run git %s", forbidden)
		}
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "fatal: boom\nhint: try again\n", want: "fatal: boom"},
		{in: "\n\n  fatal: indented\n", want: "fatal: indented"},
		{in: "", want: ""},
		{in: "\n\n", want: ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, FirstLine(tt.in))
	}
}
