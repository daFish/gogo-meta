package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/discover"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

const (
	notAGogoRepoMsg     = `Not in a gogo-meta repository. Run "gogo init" first.`
	migrationAbortedMsg = "Migration aborted: one or more target paths are occupied by a different repository"
)

type migrateMove struct{ from, to string }

// migrationPlan buckets every selected project by what the working copy needs
// doing to it. A project appears in exactly one bucket.
type migrationPlan struct {
	present   []string // directory exists and its origin matches the config
	moves     []migrateMove
	missing   []string // no directory, and the URL was found nowhere
	ambiguous []string // no directory, and the URL was found at two or more paths
	conflicts []migrateConflict
}
type migrateConflict struct {
	path  string
	found string // "" means no remote
}

func newMigrateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Move/rename working-copy directories to match the configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			code, err := runMigrate(cmd.Context(), executor.NewShellExecutor(), cwd, dryRun)
			if err != nil {
				return err
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be moved without changing anything")
	return cmd
}

func getRemoteURL(ctx context.Context, ex executor.Executor, dir string) string {
	res, err := ex.ExecuteArgs(ctx, "git", []string{"remote", "get-url", "origin"}, executor.Options{Cwd: dir})
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func mapURLsToCurrentPaths(ctx context.Context, ex executor.Executor, metaDir string, ignore []string) (urlToPath map[string]string, ambiguous map[string]bool, err error) {
	var repoPaths []string
	repoPaths, err = discover.FindGitRepos(metaDir, ignore)
	if err != nil {
		return nil, nil, err
	}
	urlToPath = map[string]string{}
	ambiguous = map[string]bool{}
	for _, rp := range repoPaths {
		url := getRemoteURL(ctx, ex, filepath.Join(metaDir, rp))
		if url == "" {
			continue
		}
		if _, ok := urlToPath[url]; ok {
			ambiguous[url] = true
			continue
		}
		urlToPath[url] = rp
	}
	return urlToPath, ambiguous, nil
}

func pruneEmptyParents(metaDir, movedFrom string) {
	root, err := filepath.Abs(metaDir)
	if err != nil {
		return
	}
	dir, err := filepath.Abs(filepath.Dir(filepath.Join(metaDir, movedFrom)))
	if err != nil {
		return
	}
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// planMigration classifies every selected project against the working copy. It
// only inspects; nothing is moved.
func planMigration(ctx context.Context, ex executor.Executor, metaDir string, cfg config.MetaConfig, selected []string) (migrationPlan, error) {
	urlToPath, ambiguousURLs, err := mapURLsToCurrentPaths(ctx, ex, metaDir, cfg.Ignore)
	if err != nil {
		return migrationPlan{}, err
	}

	var plan migrationPlan
	for _, projectPath := range selected {
		url := cfg.Projects[projectPath]
		targetDir := filepath.Join(metaDir, projectPath)

		if config.FileExists(targetDir) {
			targetRemote := getRemoteURL(ctx, ex, targetDir)
			if targetRemote != url {
				plan.conflicts = append(plan.conflicts, migrateConflict{path: projectPath, found: targetRemote})
				continue
			}
			plan.present = append(plan.present, projectPath)
			continue
		}

		switch {
		case ambiguousURLs[url]:
			plan.ambiguous = append(plan.ambiguous, projectPath)
		case urlToPath[url] != "" && urlToPath[url] != projectPath:
			plan.moves = append(plan.moves, migrateMove{from: urlToPath[url], to: projectPath})
		default:
			plan.missing = append(plan.missing, projectPath)
		}
	}
	return plan, nil
}

// applyMoves renames each working copy to its configured path and keeps the
// ignore files in step. It returns the moves it completed, stopping at the
// first filesystem error so the caller can report what did happen.
func applyMoves(metaDir string, moves []migrateMove, localProjects map[string]string) ([]migrateMove, error) {
	var applied []migrateMove

	for _, m := range moves {
		targetDir := filepath.Join(metaDir, m.to)
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
			return applied, err
		}
		if err := os.Rename(filepath.Join(metaDir, m.from), targetDir); err != nil {
			return applied, err
		}
		pruneEmptyParents(metaDir, m.from)

		if _, isLocal := localProjects[m.to]; !isLocal {
			if _, err := config.RemoveFromGitignore(metaDir, m.from); err != nil {
				return applied, err
			}
			if _, err := config.AddToGitignore(metaDir, m.to); err != nil {
				return applied, err
			}
		}
		applied = append(applied, m)
	}

	if len(applied) > 0 {
		if err := ensureLocalConfigIgnored(metaDir); err != nil {
			return applied, err
		}
		if err := syncLocalExcludes(metaDir, localProjects); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

func runMigrate(ctx context.Context, ex executor.Executor, cwd string, dryRun bool) (int, error) {
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
	cfg := result.Config
	localProjects := result.LocalProjects

	if len(cfg.Projects) == 0 {
		output.Success("Working copy already matches configuration")
		return 0, nil
	}

	desired := make([]string, 0, len(cfg.Projects))
	for p := range cfg.Projects {
		desired = append(desired, p)
	}
	sort.Strings(desired)

	plan, err := planMigration(ctx, ex, metaDir, cfg, desired)
	if err != nil {
		return 0, err
	}

	if len(plan.conflicts) > 0 {
		for _, c := range plan.conflicts {
			found := c.found
			if found == "" {
				found = "no remote"
			}
			output.ProjectStatus(c.path, "error", fmt.Sprintf("occupied by a different repository (found %s)", found))
		}
		return 0, errors.New(migrationAbortedMsg) //nolint:staticcheck // capitalized for JS parity
	}

	if len(plan.moves) == 0 && len(plan.missing) == 0 && len(plan.ambiguous) == 0 {
		output.Success("Working copy already matches configuration")
		return 0, nil
	}

	if dryRun {
		for _, m := range plan.moves {
			output.Info(fmt.Sprintf("Would move %s → %s", m.from, m.to))
		}
	} else {
		applied, err := applyMoves(metaDir, plan.moves, localProjects)
		for _, m := range applied {
			output.ProjectStatus(m.to, "success", fmt.Sprintf("moved from %s", m.from))
		}
		if err != nil {
			return 0, err
		}
	}

	for _, p := range plan.ambiguous {
		output.Warning(fmt.Sprintf("%s: multiple working-copy directories share its repository URL — resolve manually", p))
	}
	for _, p := range plan.missing {
		output.Warning(fmt.Sprintf("%s not found in working copy — run 'gogo git update' to clone", p))
	}

	if dryRun {
		output.Info(fmt.Sprintf("Dry run: %d move(s) pending", len(plan.moves)))
	}

	if len(plan.missing) > 0 || len(plan.ambiguous) > 0 {
		return 1, nil
	}
	return 0, nil
}
