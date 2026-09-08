package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/gitstate"
	"github.com/daFish/gogo-meta/internal/loop"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

const allPhasesDisabledMsg = "nothing to do: all phases disabled"

type updateOptions struct {
	Filter      filter.Options
	Parallel    bool
	Concurrency int
	DryRun      bool
	NoMigrate   bool
	NoClone     bool
	NoPull      bool
}

type outcomeClass int

const (
	outcomeOK outcomeClass = iota
	outcomeSkipped
	outcomeFailed
)

type projectOutcome struct {
	Path    string
	Class   outcomeClass
	Message string
}

// pullTarget is a project the pull phase may advance. The directory is carried
// explicitly because a project moved during this run is no longer where the
// plan found it — and during a dry run it has not moved at all.
type pullTarget struct {
	path string
	dir  string
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Converge the working copy on the configuration: move, clone and fast-forward",
		Long: `Bring every configured project into the state the configuration describes.

Projects that moved are renamed to their configured path, projects that are
missing are cloned, and the rest are fast-forwarded to their upstream. Nothing
is merged, rebased or stashed, and no branch is switched.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			filterOpts, err := resolveFilterOptions(cmd)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			opts := updateOptions{
				Filter:      filterOpts,
				Parallel:    getBoolFlag(cmd, "parallel"),
				Concurrency: getIntFlag(cmd, "concurrency"),
				DryRun:      getBoolFlag(cmd, "dry-run"),
				NoMigrate:   getBoolFlag(cmd, "no-migrate"),
				NoClone:     getBoolFlag(cmd, "no-clone"),
				NoPull:      getBoolFlag(cmd, "no-pull"),
			}

			code, err := runUpdate(cmd.Context(), executor.NewShellExecutor(), cwd, opts)
			if err != nil {
				return err
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Show what would be done without changing anything")
	cmd.Flags().Bool("no-migrate", false, "Do not move projects that are checked out at another path")
	cmd.Flags().Bool("no-clone", false, "Do not clone projects that are missing")
	cmd.Flags().Bool("no-pull", false, "Do not fast-forward projects that are present")
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

// outcomeRecorder collects one outcome per project. A project is classified by
// exactly one phase, so a second write is a bug rather than an update.
type outcomeRecorder struct {
	byPath map[string]projectOutcome
	order  []string
}

func newOutcomeRecorder() *outcomeRecorder {
	return &outcomeRecorder{byPath: map[string]projectOutcome{}}
}

func (r *outcomeRecorder) record(path string, class outcomeClass, message string) {
	if _, seen := r.byPath[path]; seen {
		return
	}
	r.byPath[path] = projectOutcome{Path: path, Class: class, Message: message}
	r.order = append(r.order, path)
}

func (r *outcomeRecorder) outcomes() []projectOutcome {
	sorted := append([]string(nil), r.order...)
	sort.Strings(sorted)

	list := make([]projectOutcome, 0, len(sorted))
	for _, path := range sorted {
		list = append(list, r.byPath[path])
	}
	return list
}

func runUpdate(ctx context.Context, ex executor.Executor, cwd string, opts updateOptions) (int, error) {
	if opts.NoMigrate && opts.NoClone && opts.NoPull {
		return 0, errors.New(allPhasesDisabledMsg)
	}

	metaDir, err := config.GetMetaDir(cwd)
	if err != nil {
		return 0, err
	}
	if metaDir == "" {
		return 0, errors.New(notAGogoRepoMsg) //nolint:staticcheck // capitalized for JS parity
	}

	result, err := config.ReadMetaConfig(cwd, nil)
	if err != nil {
		return 0, err
	}
	printOverlayInfo(result)
	cfg := result.Config

	// Filter-independent, and done before anything else: dropping a project
	// from .gogo.local must prune its stale exclude entry either way.
	if err := syncLocalExcludes(metaDir, result.LocalProjects); err != nil {
		return 0, err
	}

	selected := filter.Apply(config.GetProjectPaths(cfg), opts.Filter)
	if len(selected) == 0 {
		output.Warning("No projects match the specified filters")
		return 0, nil
	}

	plan, err := planMigration(ctx, ex, metaDir, cfg, selected)
	if err != nil {
		return 0, err
	}

	recorder := newOutcomeRecorder()

	// The pull set is built once, as each phase establishes that a project is
	// present and correctly placed. It is never re-planned from disk: a re-plan
	// after the moves would reclassify projects that a phase already reported.
	pullTargets := make([]pullTarget, 0, len(selected))
	for _, path := range plan.present {
		pullTargets = append(pullTargets, pullTarget{path: path, dir: filepath.Join(metaDir, path)})
	}

	pullTargets = runMovePhase(recorder, plan, opts, metaDir, result.LocalProjects, pullTargets)
	runClonePhase(ctx, ex, recorder, plan, opts, metaDir, cfg)
	runPullPhase(ctx, ex, recorder, opts, pullTargets)

	return reportUpdate(recorder.outcomes(), opts), nil
}

func runMovePhase(recorder *outcomeRecorder, plan migrationPlan, opts updateOptions, metaDir string, localProjects map[string]string, pullTargets []pullTarget) []pullTarget {
	for _, c := range plan.conflicts {
		found := c.found
		if found == "" {
			found = "no remote"
		}
		recorder.record(c.path, outcomeFailed, fmt.Sprintf("occupied by a different repository (found %s)", found))
	}

	if len(plan.moves) == 0 {
		return pullTargets
	}

	switch {
	case opts.NoMigrate:
		for _, m := range plan.moves {
			recorder.record(m.to, outcomeFailed, fmt.Sprintf("found at %s — run without --no-migrate to move it", m.from))
		}

	case len(plan.conflicts) > 0:
		// A conflict aborts the whole move phase, exactly as gogo migrate does:
		// the pending renames are reported, not attempted.
		for _, m := range plan.moves {
			recorder.record(m.to, outcomeFailed, "not moved: migration aborted by a conflict")
		}

	case opts.DryRun:
		for _, m := range plan.moves {
			recorder.record(m.to, outcomeOK, fmt.Sprintf("would move from %s", m.from))
			// Still at its old path during a dry run, so that is where the pull
			// preview has to look.
			pullTargets = append(pullTargets, pullTarget{path: m.to, dir: filepath.Join(metaDir, m.from)})
		}

	default:
		applied, err := applyMoves(metaDir, plan.moves, localProjects)
		for _, m := range applied {
			recorder.record(m.to, outcomeOK, fmt.Sprintf("moved from %s", m.from))
			pullTargets = append(pullTargets, pullTarget{path: m.to, dir: filepath.Join(metaDir, m.to)})
		}
		if err != nil {
			for _, m := range plan.moves {
				recorder.record(m.to, outcomeFailed, fmt.Sprintf("move failed: %v", err))
			}
		}
	}

	return pullTargets
}

func runClonePhase(ctx context.Context, ex executor.Executor, recorder *outcomeRecorder, plan migrationPlan, opts updateOptions, metaDir string, cfg config.MetaConfig) {
	for _, path := range plan.ambiguous {
		recorder.record(path, outcomeFailed, "multiple working-copy directories share its repository URL — resolve manually")
	}

	if len(plan.missing) == 0 {
		return
	}

	if opts.NoClone {
		for _, path := range plan.missing {
			recorder.record(path, outcomeSkipped, "not cloned (--no-clone)")
		}
		return
	}

	if opts.DryRun {
		for _, path := range plan.missing {
			recorder.record(path, outcomeOK, "would clone")
		}
		return
	}

	targets := make([]cloneTarget, 0, len(plan.missing))
	urls := make([]string, 0, len(plan.missing))
	for _, path := range plan.missing {
		targets = append(targets, cloneTarget{Path: path, URL: cfg.Projects[path]})
		urls = append(urls, cfg.Projects[path])
	}
	warnUnverifiedSSHHosts(urls)

	// A freshly cloned repository is current by construction, so it is not
	// added to the pull set.
	for _, o := range cloneMissing(ctx, ex, metaDir, targets, opts.Parallel, opts.Concurrency) {
		if o.Err == "" {
			recorder.record(o.Path, outcomeOK, "cloned")
			continue
		}
		recorder.record(o.Path, outcomeFailed, o.Err)
	}
}

func runPullPhase(ctx context.Context, ex executor.Executor, recorder *outcomeRecorder, opts updateOptions, targets []pullTarget) {
	if len(targets) == 0 {
		return
	}

	if opts.NoPull {
		for _, t := range targets {
			recorder.record(t.path, outcomeOK, "not pulled (--no-pull)")
		}
		return
	}

	dirs := make([]string, len(targets))
	byDir := make(map[string]pullTarget, len(targets))
	for i, t := range targets {
		dirs[i] = t.dir
		byDir[t.dir] = t
	}

	results := loop.RunEach(ctx, dirs, opts.Parallel, opts.Concurrency, func(ctx context.Context, dir string) (*executor.Result, error) {
		class, message := pullProject(ctx, ex, dir, opts.DryRun)
		// The outcome travels in the result rather than being recorded here, so
		// concurrent workers never write to the recorder.
		return &executor.Result{ExitCode: int(class), Stdout: message}, nil
	})

	for i, r := range results {
		target := byDir[dirs[i]]
		recorder.record(target.path, outcomeClass(r.Result.ExitCode), r.Result.Stdout)
	}
}

// pullProject applies the pull policy to one working copy: inspect, fetch, and
// at most one fast-forward. It never merges, rebases, stashes or switches
// branches.
func pullProject(ctx context.Context, ex executor.Executor, dir string, dryRun bool) (class outcomeClass, message string) {
	status, err := gitstate.Inspect(ctx, ex, dir)
	if err != nil {
		return outcomeFailed, fmt.Sprintf("inspect failed: %v", err)
	}

	switch {
	case status.Detached:
		return outcomeSkipped, "detached HEAD"
	case status.Upstream == "":
		return outcomeSkipped, "no upstream branch"
	}

	dirtyNote := ""
	if status.Dirty {
		dirtyNote = " (working tree has local changes)"
	}

	if dryRun {
		// A dry run inspects and stops. It cannot know whether the project is
		// behind without fetching, and fetching is a change to the repository.
		return outcomeOK, "would pull" + dirtyNote
	}

	fetched, err := gitstate.Fetch(ctx, ex, dir)
	if err != nil {
		return outcomeFailed, fmt.Sprintf("fetch failed: %v", err)
	}
	if fetched.TimedOut {
		return outcomeFailed, fmt.Sprintf("fetch timed out after %s", executor.DefaultTimeout)
	}
	if fetched.ExitCode != 0 {
		return outcomeFailed, "fetch failed: " + gitstate.FirstLine(fetched.Stderr)
	}

	// An unborn branch has no HEAD to compare against its upstream — rev-list
	// fails outright — but the fast-forward is exactly what converges it.
	if status.Unborn() {
		return finishFastForward(ctx, ex, dir, "updated", dirtyNote)
	}

	ahead, behind, err := gitstate.AheadBehind(ctx, ex, dir)
	if err != nil {
		return outcomeFailed, fmt.Sprintf("compare failed: %v", err)
	}

	switch {
	case behind == 0 && ahead == 0:
		return outcomeOK, "up to date" + dirtyNote
	case behind == 0:
		return outcomeOK, fmt.Sprintf("up to date (ahead %d, not pushed)", ahead) + dirtyNote
	case ahead > 0:
		return outcomeFailed, fmt.Sprintf("diverged from upstream (ahead %d, behind %d) — merge or rebase manually", ahead, behind)
	}

	return finishFastForward(ctx, ex, dir, fmt.Sprintf("updated (%d commit(s))", behind), dirtyNote)
}

// finishFastForward runs the fast-forward and turns git's own refusal - which
// is what guards a dirty working tree - into a failed outcome.
func finishFastForward(ctx context.Context, ex executor.Executor, dir, successMessage, dirtyNote string) (class outcomeClass, message string) {
	merged, err := gitstate.FastForward(ctx, ex, dir)
	if err != nil {
		return outcomeFailed, fmt.Sprintf("fast-forward failed: %v", err)
	}
	if merged.TimedOut {
		return outcomeFailed, fmt.Sprintf("fast-forward timed out after %s", executor.DefaultTimeout)
	}
	if merged.ExitCode != 0 {
		return outcomeFailed, "fast-forward refused: " + gitstate.FirstLine(merged.Stderr) + dirtyNote
	}
	return outcomeOK, successMessage + dirtyNote
}

func reportUpdate(outcomes []projectOutcome, opts updateOptions) int {
	var failedProjects []string
	okCount, skippedCount := 0, 0

	for _, o := range outcomes {
		switch o.Class {
		case outcomeOK:
			output.ProjectStatus(o.Path, "success", o.Message)
			okCount++
		case outcomeSkipped:
			output.Warning(fmt.Sprintf("%s %s", o.Path, o.Message))
			skippedCount++
		case outcomeFailed:
			output.ProjectStatus(o.Path, "error", o.Message)
			failedProjects = append(failedProjects, o.Path)
		}
	}

	if opts.DryRun {
		output.Info("Dry run: nothing was changed. Clone and fast-forward results cannot be known without contacting the remotes.")
	}

	output.Summary(output.SummaryData{
		Success:        okCount + skippedCount,
		Failed:         len(failedProjects),
		Total:          len(outcomes),
		FailedProjects: failedProjects,
	})

	if len(failedProjects) > 0 {
		return 1
	}
	return 0
}
