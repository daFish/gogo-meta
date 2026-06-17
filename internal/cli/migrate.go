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
type migrateConflict struct {
	path  string
	found string // "" means no remote
}

func newMigrateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Move/rename working-copy directories to match the configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			code, err := runMigrate(executor.NewShellExecutor(), cwd, dryRun)
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

func getRemoteURL(ex executor.Executor, dir string) string {
	res, err := ex.Execute(context.Background(), "git remote get-url origin", executor.Options{Cwd: dir})
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func mapURLsToCurrentPaths(ex executor.Executor, metaDir string, ignore []string) (urlToPath map[string]string, ambiguous map[string]bool, err error) {
	var repoPaths []string
	repoPaths, err = discover.FindGitRepos(metaDir, ignore)
	if err != nil {
		return nil, nil, err
	}
	urlToPath = map[string]string{}
	ambiguous = map[string]bool{}
	for _, rp := range repoPaths {
		url := getRemoteURL(ex, filepath.Join(metaDir, rp))
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

func runMigrate(ex executor.Executor, cwd string, dryRun bool) (int, error) {
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

	if len(cfg.Projects) == 0 {
		output.Success("Working copy already matches configuration")
		return 0, nil
	}

	urlToPath, ambiguousURLs, err := mapURLsToCurrentPaths(ex, metaDir, cfg.Ignore)
	if err != nil {
		return 0, err
	}

	desired := make([]string, 0, len(cfg.Projects))
	for p := range cfg.Projects {
		desired = append(desired, p)
	}
	sort.Strings(desired)

	var moves []migrateMove
	var missing, ambiguous []string
	var conflicts []migrateConflict

	for _, projectPath := range desired {
		url := cfg.Projects[projectPath]
		targetDir := filepath.Join(metaDir, projectPath)

		if config.FileExists(targetDir) {
			targetRemote := getRemoteURL(ex, targetDir)
			if targetRemote != url {
				conflicts = append(conflicts, migrateConflict{path: projectPath, found: targetRemote})
			}
			continue
		}

		switch {
		case ambiguousURLs[url]:
			ambiguous = append(ambiguous, projectPath)
		case urlToPath[url] != "" && urlToPath[url] != projectPath:
			moves = append(moves, migrateMove{from: urlToPath[url], to: projectPath})
		default:
			missing = append(missing, projectPath)
		}
	}

	if len(conflicts) > 0 {
		for _, c := range conflicts {
			found := c.found
			if found == "" {
				found = "no remote"
			}
			output.ProjectStatus(c.path, "error", fmt.Sprintf("occupied by a different repository (found %s)", found))
		}
		return 0, errors.New(migrationAbortedMsg) //nolint:staticcheck // capitalized for JS parity
	}

	if len(moves) == 0 && len(missing) == 0 && len(ambiguous) == 0 {
		output.Success("Working copy already matches configuration")
		return 0, nil
	}

	for _, m := range moves {
		if dryRun {
			output.Info(fmt.Sprintf("Would move %s → %s", m.from, m.to))
			continue
		}
		targetDir := filepath.Join(metaDir, m.to)
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
			return 0, err
		}
		if err := os.Rename(filepath.Join(metaDir, m.from), targetDir); err != nil {
			return 0, err
		}
		pruneEmptyParents(metaDir, m.from)
		if _, err := config.RemoveFromGitignore(metaDir, m.from); err != nil {
			return 0, err
		}
		if _, err := config.AddToGitignore(metaDir, m.to); err != nil {
			return 0, err
		}
		output.ProjectStatus(m.to, "success", fmt.Sprintf("moved from %s", m.from))
	}

	for _, p := range ambiguous {
		output.Warning(fmt.Sprintf("%s: multiple working-copy directories share its repository URL — resolve manually", p))
	}
	for _, p := range missing {
		output.Warning(fmt.Sprintf("%s not found in working copy — run 'gogo git update' to clone", p))
	}

	if dryRun {
		output.Info(fmt.Sprintf("Dry run: %d move(s) pending", len(moves)))
		return 0, nil
	}

	if len(missing) > 0 || len(ambiguous) > 0 {
		return 1, nil
	}
	return 0, nil
}
