package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
// Returns (changed, err); (false, nil) if metaDir has no .git directory. If the
// managed header exists without its footer, the file is left untouched and an
// error is returned — never guess where a broken block ends.
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

	kept, ok := stripManagedBlock(original)
	if !ok {
		return false, fmt.Errorf(
			"%s: gogo-meta managed block header found without its footer line (%q) — file left untouched, restore the footer or remove the block manually",
			excludePath, gitExcludeManagedFooter)
	}

	result := assembleExclude(kept, uniq)
	if result == original {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(excludePath, []byte(result), 0o644) // #nosec G306
}

// stripManagedBlock returns content's lines with the gogo-meta-managed block
// (header..footer inclusive) removed. ok is false when the header exists but the
// footer is missing — the block boundary is unknowable, so callers must not
// rewrite the file (doing so would eat the user's lines below the header).
func stripManagedBlock(content string) (out []string, ok bool) {
	if content == "" {
		return nil, true
	}
	lines := strings.Split(content, "\n")
	out = make([]string, 0, len(lines))
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
	if inBlock {
		return nil, false
	}
	return out, true
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
