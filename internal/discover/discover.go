// Package discover walks a directory tree to locate git repositories.
package discover

import (
	"os"
	"path/filepath"
	"sort"
)

const gitDir = ".git"

func toPosixRelative(rootDir, dir string) string {
	rel, err := filepath.Rel(rootDir, dir)
	if err != nil {
		rel = dir
	}
	return filepath.ToSlash(rel)
}

// FindGitRepos walks rootDir and returns the paths (relative to rootDir,
// POSIX-style) of every directory that is a git repository. The root itself is
// never reported, discovered repositories are not descended into, and any
// directory whose base name appears in ignore is skipped.
func FindGitRepos(rootDir string, ignore []string) ([]string, error) {
	ignoreSet := make(map[string]bool, len(ignore))
	for _, name := range ignore {
		ignoreSet[name] = true
	}

	var repos []string

	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		isRepo := false
		for _, e := range entries {
			if e.Name() == gitDir {
				isRepo = true
				break
			}
		}
		if isRepo && dir != rootDir {
			repos = append(repos, toPosixRelative(rootDir, dir))
			return
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if e.Name() == gitDir || ignoreSet[e.Name()] {
				continue
			}
			walk(filepath.Join(dir, e.Name()))
		}
	}

	walk(rootDir)
	sort.Strings(repos)
	return repos, nil
}
