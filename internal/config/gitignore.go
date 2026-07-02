package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// appendUniqueLine appends entry as its own line to the file at path, creating
// the file if absent. Returns true if the entry was added, false if a line
// matching entry (after trimming) already existed.
func appendUniqueLine(path, entry string) (bool, error) {
	if !FileExists(path) {
		return true, os.WriteFile(path, []byte(entry+"\n"), 0o644)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			return false, nil
		}
	}

	suffix := ""
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		suffix = "\n"
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(suffix + entry + "\n"); err != nil {
		return false, err
	}
	return true, nil
}

// removeLine removes the line matching entry (after trimming) from the file at
// path. Returns true if a line was removed, false if the file is absent or
// contained no matching line.
func removeLine(path, entry string) (bool, error) {
	if !FileExists(path) {
		return false, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != entry {
			filtered = append(filtered, line)
		}
	}

	if len(filtered) == len(lines) {
		return false, nil
	}

	return true, os.WriteFile(path, []byte(strings.Join(filtered, "\n")), 0o644) // #nosec G703
}

// AddToGitignore adds an entry to the .gitignore file in the given directory.
// Returns true if the entry was added, false if it already existed.
// Creates the .gitignore file if it doesn't exist.
func AddToGitignore(metaDir, entry string) (bool, error) {
	return appendUniqueLine(filepath.Join(metaDir, ".gitignore"), entry)
}

// RemoveFromGitignore removes the line matching entry (after trimming) from the
// .gitignore in metaDir. Returns true if a line was removed, false if the file
// is absent or contained no matching line.
func RemoveFromGitignore(metaDir, entry string) (bool, error) {
	return removeLine(filepath.Join(metaDir, ".gitignore"), entry)
}

// Marker lines delimiting the gogo-meta-managed block in .git/info/exclude.
// gogo owns everything between them; content outside is the user's and untouched.
const (
	gitExcludeManagedHeader = "# >>> gogo-meta managed (.gogo.local project directories) — do not edit"
	gitExcludeManagedFooter = "# <<< gogo-meta managed"
)

// SyncGitExcludeManagedBlock rewrites the gogo-meta-managed block in
// <metaDir>/.git/info/exclude so it lists exactly entries (deduped, sorted),
// leaving any user content outside the markers untouched. An empty entries slice
// removes the block entirely (fixing the leak when a project leaves .gogo.local).
// Returns (changed, err); (false, nil) if metaDir has no .git directory.
//
// ponytail: assumes .git is a directory (the normal umbrella-repo case). Worktrees
// and submodules use a `.git` *file*; resolve via `git rev-parse --git-path info/exclude`
// in the CLI layer if those must be supported.
func SyncGitExcludeManagedBlock(metaDir string, entries []string) (bool, error) {
	gitDir := filepath.Join(metaDir, ".git")
	fi, err := os.Stat(gitDir)
	if err != nil || !fi.IsDir() {
		return false, nil //nolint:nilerr // absent/file .git → nothing to do
	}
	excludePath := filepath.Join(gitDir, "info", "exclude")

	var original string
	if FileExists(excludePath) {
		b, rerr := os.ReadFile(excludePath)
		if rerr != nil {
			return false, rerr
		}
		original = string(b)
	}

	uniq := dedupeSorted(entries)

	// Nothing to add and no existing block → leave the file exactly as-is.
	if len(uniq) == 0 && !strings.Contains(original, gitExcludeManagedHeader) {
		return false, nil
	}

	result := assembleExclude(stripManagedBlock(original), uniq)
	if result == original {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(excludePath, []byte(result), 0o644) // #nosec G306
}

// stripManagedBlock returns content's lines with the gogo-meta-managed block
// (header..footer inclusive) removed.
func stripManagedBlock(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, ln := range lines {
		switch strings.TrimSpace(ln) {
		case gitExcludeManagedHeader:
			inBlock = true
			continue
		case gitExcludeManagedFooter:
			if inBlock {
				inBlock = false
				continue
			}
		}
		if !inBlock {
			out = append(out, ln)
		}
	}
	return out
}

// assembleExclude joins the user's kept lines with a freshly built managed block.
func assembleExclude(kept, entries []string) string {
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	var b strings.Builder
	for _, ln := range kept {
		b.WriteString(ln)
		b.WriteString("\n")
	}
	if len(entries) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(gitExcludeManagedHeader + "\n")
		for _, e := range entries {
			b.WriteString(e + "\n")
		}
		b.WriteString(gitExcludeManagedFooter + "\n")
	}
	return b.String()
}

// dedupeSorted returns the non-empty entries, deduplicated and sorted.
func dedupeSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
