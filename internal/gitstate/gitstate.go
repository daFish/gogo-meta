// Package gitstate inspects and fast-forwards a git working copy without ever
// creating a merge commit, rebasing, stashing or switching branches.
//
// It reads machine-readable plumbing rather than porcelain prose: git localizes
// its human-facing messages, and the executor cannot pin LC_ALL without
// replacing the whole environment, so no output parsed here is translatable.
package gitstate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/daFish/gogo-meta/internal/executor"
)

const (
	detachedMarker = "(detached)"
	initialMarker  = "(initial)"
)

// Status is what `git status --porcelain=v2 --branch` reports about a working
// copy.
type Status struct {
	Head     string // commit id; empty on an unborn branch
	Branch   string // empty when detached
	Detached bool
	Upstream string // empty when the branch has no upstream
	Dirty    bool   // any changed, renamed, unmerged or untracked entry
}

// Unborn reports a branch that has no commit yet, which cannot be compared
// against its upstream.
func (s Status) Unborn() bool { return s.Head == "" }

// ParsePorcelainV2 reads the branch headers and entry lines of
// `git status --porcelain=v2 --branch`.
//
// The `# branch.ab` header is deliberately ignored: it is computed against the
// remote-tracking ref as it stood before any fetch, so it is stale exactly when
// it matters. AheadBehind is used instead.
func ParsePorcelainV2(out string) Status {
	var status Status

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "# ") {
			// 1 = changed, 2 = renamed or copied, u = unmerged, ? = untracked.
			switch line[0] {
			case '1', '2', 'u', '?':
				status.Dirty = true
			}
			continue
		}

		header, value, found := strings.Cut(strings.TrimPrefix(line, "# "), " ")
		if !found {
			continue
		}
		switch header {
		case "branch.oid":
			if value != initialMarker {
				status.Head = value
			}
		case "branch.head":
			if value == detachedMarker {
				status.Detached = true
				continue
			}
			status.Branch = value
		case "branch.upstream":
			status.Upstream = value
		}
	}

	return status
}

// Inspect reports the state of the working copy in dir. It changes nothing.
func Inspect(ctx context.Context, ex executor.Executor, dir string) (Status, error) {
	result, err := ex.ExecuteArgs(ctx, "git", []string{"status", "--porcelain=v2", "--branch"}, executor.Options{Cwd: dir})
	if err != nil {
		return Status{}, err
	}
	if result.ExitCode != 0 {
		return Status{}, fmt.Errorf("git status failed: %s", FirstLine(result.Stderr))
	}
	return ParsePorcelainV2(result.Stdout), nil
}

// Fetch updates the remote-tracking refs of dir. It does not touch the working
// tree or the current branch.
func Fetch(ctx context.Context, ex executor.Executor, dir string) (*executor.Result, error) {
	return ex.ExecuteArgs(ctx, "git", []string{"fetch"}, executor.Options{Cwd: dir})
}

// AheadBehind counts the commits the current branch has that its upstream does
// not, and the other way round.
func AheadBehind(ctx context.Context, ex executor.Executor, dir string) (ahead, behind int, err error) {
	result, err := ex.ExecuteArgs(ctx, "git", []string{"rev-list", "--left-right", "--count", "HEAD...@{u}"}, executor.Options{Cwd: dir})
	if err != nil {
		return 0, 0, err
	}
	if result.ExitCode != 0 {
		return 0, 0, fmt.Errorf("%s", FirstLine(result.Stderr))
	}

	fields := strings.Fields(result.Stdout)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output %q", strings.TrimSpace(result.Stdout))
	}
	if ahead, err = strconv.Atoi(fields[0]); err != nil {
		return 0, 0, fmt.Errorf("unexpected rev-list output %q", strings.TrimSpace(result.Stdout))
	}
	if behind, err = strconv.Atoi(fields[1]); err != nil {
		return 0, 0, fmt.Errorf("unexpected rev-list output %q", strings.TrimSpace(result.Stdout))
	}
	return ahead, behind, nil
}

// FastForward advances the current branch to its upstream. git refuses when the
// move is not a fast-forward, and when local or untracked changes would be
// overwritten, so it never produces conflict markers.
func FastForward(ctx context.Context, ex executor.Executor, dir string) (*executor.Result, error) {
	return ex.ExecuteArgs(ctx, "git", []string{"merge", "--ff-only", "@{u}"}, executor.Options{Cwd: dir})
}

// FirstLine reduces git's stderr to its first non-empty line, which is the part
// worth putting on a per-project status line.
func FirstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
